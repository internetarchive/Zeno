package archiver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/CorentinB/warc"
	"github.com/dustin/go-humanize"
	"github.com/gabriel-vasile/mimetype"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/domainscrawl"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/internetarchive/Zeno/pkg/models"
)

func init() {
	// We intentionally set the limit to 0 to disable the limit on the number of bytes the
	// mimetype detection can accept. We limit the number of bytes that we will give to it
	// in the processBody function instead.
	mimetype.SetLimit(0)
}

type archiver struct {
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	inputCh  chan *models.Item
	outputCh chan *models.Item

	Client          *warc.CustomHTTPClient
	ClientWithProxy *warc.CustomHTTPClient
}

var (
	globalArchiver *archiver
	once           sync.Once
	logger         *log.FieldedLogger
)

// This functions starts the archiver responsible for capturing the URLs
func Start(inputChan, outputChan chan *models.Item) error {
	var done bool

	log.Start()
	logger = log.NewFieldedLogger(&log.Fields{
		"component": "archiver",
	})

	stats.Init()

	once.Do(func() {
		ctx, cancel := context.WithDeadlineCause(context.Background(), time.Now().Add(1*time.Minute), errors.New("archiver context deadline exceeded"))
		globalArchiver = &archiver{
			ctx:      ctx,
			cancel:   cancel,
			inputCh:  inputChan,
			outputCh: outputChan,
		}
		logger.Debug("initialized")

		// Setup WARC writing HTTP clients
		startWARCWriter()
		go watchWARCWritingQueue(250 * time.Millisecond)
		go globalArchiver.run()
		logger.Info("started")
		done = true
	})

	if !done {
		return ErrArchiverAlreadyInitialized
	}

	return nil
}

func Stop() {
	if globalArchiver != nil {
		globalArchiver.cancel()
		globalArchiver.wg.Wait()
		logger.Debug("all routines stopped")

		// Wait for the WARC writing to finish
		stopLocalWatcher := make(chan struct{})
		go func() {
			for {
				select {
				case <-stopLocalWatcher:
					return
				case <-time.After(1 * time.Second):
					logger.Debug("waiting for WARC writing to finish", "queue_size", GetWARCWritingQueueSize(), "bytes_written", humanize.Bytes(uint64(warc.DataTotal.Value())))
				}
			}
		}()
		globalArchiver.Client.WaitGroup.Wait()
		stopLocalWatcher <- struct{}{}
		logger.Debug("WARC writing finished")
		globalArchiver.Client.Close()
		if globalArchiver.ClientWithProxy != nil {
			globalArchiver.ClientWithProxy.WaitGroup.Wait()
			globalArchiver.ClientWithProxy.Close()
		}

		watchWARCWritingQueueCancel()

		logger.Info("stopped")
	}
}

func (a *archiver) run() {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "archiver.run",
	})

	a.wg.Add(1)
	defer a.wg.Done()

	// Create a wait group to track all the goroutines spawned by run function
	var runWg sync.WaitGroup

	// Guard to limit the number of concurrent archiver routines
	guard := make(chan struct{}, config.Get().WorkersCount)

	// Subscribe to the pause controler
	controlChans := pause.Subscribe()
	defer pause.Unsubscribe(controlChans)

	for {
		select {
		case <-a.ctx.Done():
			logger.Debug("shutting down")
			runWg.Wait()
			return
		case <-controlChans.PauseCh:
			logger.Debug("received pause event")
			controlChans.ResumeCh <- struct{}{}
			logger.Debug("received resume event")
		case rxItem, ok := <-a.inputCh:
			if ok {
				logger.Debug("received seed item", "item", rxItem.GetShortID(), "depth", rxItem.GetDepth(), "hops", rxItem.GetURL().GetHops())
				guard <- struct{}{}
				runWg.Add(1)
				stats.ArchiverRoutinesIncr()
				go func(item *models.Item) {
					defer runWg.Done()
					defer func() { <-guard }()
					defer stats.ArchiverRoutinesDecr()

					if err := item.CheckConsistency(); err != nil {
						panic(fmt.Sprintf("item consistency check failed with err: %s, item id %s", err.Error(), item.GetShortID()))
					}

					if item.GetStatus() != models.ItemPreProcessed && item.GetStatus() != models.ItemGotRedirected && item.GetStatus() != models.ItemGotChildren {
						logger.Debug("skipping item", "item", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops(), "status", item.GetStatus().String())
					} else {
						a.archive(item)
					}

					select {
					case <-a.ctx.Done():
						logger.Debug("aborting item due to stop", "item", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops())
						return
					case a.outputCh <- item:
						return
					}
				}(rxItem)
			} else {
				a.cancel()
			}
		}
	}
}

func (a *archiver) archive(seed *models.Item) {
	// TODO: rate limiting handling
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "archiver.archive",
	})

	// Create a wait group to track all the goroutines spawned by archive function
	var archiveWg sync.WaitGroup

	// Guard to limit the number of concurrent archive routines
	guard := make(chan struct{}, config.Get().MaxConcurrentAssets)

	// Subscribe to the pause controler
	controlChans := pause.Subscribe()
	defer pause.Unsubscribe(controlChans)

	items, err := seed.GetNodesAtLevel(seed.GetMaxDepth())
	if err != nil {
		logger.Error("unable to get nodes at level", "err", err.Error(), "seed_id", seed.GetShortID())
		panic(err)
	}

	for i := range items {
		select {
		case <-a.ctx.Done():
			logger.Debug("aborting archiving of item due to stop", "seed_id", seed.GetShortID(), "item_id", items[i].GetShortID(), "depth", items[i].GetDepth())
			items[i].SetStatus(models.ItemFailed)
			continue
		case <-controlChans.PauseCh:
			logger.Debug("received pause event")
			controlChans.ResumeCh <- struct{}{}
			logger.Debug("received resume event")
		default:
		}

		if items[i].GetStatus() != models.ItemPreProcessed {
			logger.Debug("skipping item", "seed_id", seed.GetShortID(), "item_id", items[i].GetShortID(), "status", items[i].GetStatus().String(), "depth", items[i].GetDepth())
			continue
		}

		guard <- struct{}{}
		archiveWg.Add(1)
		stats.ArchiverRoutinesIncr()
		go func(item *models.Item) {
			defer archiveWg.Done()
			defer func() { <-guard }()
			defer stats.URLsCrawledIncr()
			defer stats.ArchiverRoutinesDecr()

			var (
				err  error
				resp *http.Response
			)

			select {
			case <-a.ctx.Done():
				logger.Debug("aborting archiving of child due to stop", "seed_id", seed.GetShortID(), "item_id", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops())
				item.SetStatus(models.ItemFailed)
				return
			default:
				// Execute the request
				req := item.GetURL().GetRequest().WithContext(context.TODO())
				if req == nil {
					panic("request is nil")
				}
				if config.Get().Proxy != "" {
					resp, err = a.ClientWithProxy.Do(req)
				} else {
					resp, err = a.Client.Do(req)
				}
				if err != nil {
					logger.Error("unable to execute request", "err", err.Error(), "seed_id", seed.GetShortID(), "item_id", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops())
					item.SetStatus(models.ItemFailed)
					return
				}

				// Set the response in the URL
				item.GetURL().SetResponse(resp)

				// Process the body
				err = ProcessBody(item.GetURL(), config.Get().DisableAssetsCapture, domainscrawl.Enabled(), config.Get().MaxHops, config.Get().WARCTempDir)
				if err != nil {
					logger.Error("unable to process body", "err", err.Error(), "item_id", item.GetShortID(), "seed_id", seed.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops())
					item.SetStatus(models.ItemFailed)
					return
				}

				stats.HTTPReturnCodesIncr(strconv.Itoa(resp.StatusCode))

				logger.Info("url archived", "url", item.GetURL().String(), "seed_id", seed.GetShortID(), "item_id", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops(), "status", resp.StatusCode)

				item.SetStatus(models.ItemArchived)

				return
			}
		}(items[i])
	}

	// Wait for all goroutines to finish
	archiveWg.Wait()
	return
}
