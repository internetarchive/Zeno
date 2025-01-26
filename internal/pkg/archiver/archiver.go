package archiver

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/CorentinB/warc"
	"github.com/gabriel-vasile/mimetype"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
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
		ctx, cancel := context.WithCancel(context.Background())
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

		globalArchiver.wg.Add(1)
		go run()
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

		// Wait for the WARC writing to finish
		globalArchiver.Client.WaitGroup.Wait()
		globalArchiver.Client.Close()
		if globalArchiver.ClientWithProxy != nil {
			globalArchiver.ClientWithProxy.WaitGroup.Wait()
			globalArchiver.ClientWithProxy.Close()
		}

		watchWARCWritingQueueCancel()

		logger.Info("stopped")
	}
}

func run() {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "archiver.run",
	})

	defer globalArchiver.wg.Done()

	// Create a context to manage goroutines
	ctx, cancel := context.WithCancel(globalArchiver.ctx)
	defer cancel()

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Guard to limit the number of concurrent archiver routines
	guard := make(chan struct{}, config.Get().WorkersCount)

	// Subscribe to the pause controler
	controlChans := pause.Subscribe()
	defer pause.Unsubscribe(controlChans)

	for {
		select {
		case <-controlChans.PauseCh:
			logger.Debug("received pause event")
			controlChans.ResumeCh <- struct{}{}
			logger.Debug("received resume event")
		case item, ok := <-globalArchiver.inputCh:
			if ok {
				logger.Debug("received item", "item", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops())
				guard <- struct{}{}
				wg.Add(1)
				stats.ArchiverRoutinesIncr()
				go func(ctx context.Context) {
					defer wg.Done()
					defer func() { <-guard }()
					defer stats.ArchiverRoutinesDecr()

					if err := item.CheckConsistency(); err != nil {
						panic(fmt.Sprintf("item consistency check failed with err: %s, item id %s", err.Error(), item.GetShortID()))
					}

					if item.GetStatus() != models.ItemPreProcessed && item.GetStatus() != models.ItemGotRedirected && item.GetStatus() != models.ItemGotChildren {
						logger.Debug("skipping item", "item", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops(), "status", item.GetStatus().String())
					} else {
						archive(item)
					}

					select {
					case globalArchiver.outputCh <- item:
					case <-ctx.Done():
						logger.Debug("aborting item due to stop", "item", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops())
						return
					}
				}(ctx)
			}
		case <-globalArchiver.ctx.Done():
			logger.Debug("shutting down")
			wg.Wait()
			return
		}
		stats.WarcWritingQueueSizeSet(int64(GetWARCWritingQueueSize()))
	}
}

func archive(seed *models.Item) {
	// TODO: rate limiting handling
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "archiver.archive",
	})

	var (
		guard = make(chan struct{}, config.Get().MaxConcurrentAssets)
		wg    sync.WaitGroup
	)

	items, err := seed.GetNodesAtLevel(seed.GetMaxDepth())
	if err != nil {
		logger.Error("unable to get nodes at level", "err", err.Error(), "seed_id", seed.GetShortID())
		panic(err)
	}

	for i := range items {
		if items[i].GetStatus() != models.ItemPreProcessed {
			logger.Debug("skipping item", "seed_id", seed.GetShortID(), "item_id", items[i].GetShortID(), "status", items[i].GetStatus().String(), "depth", items[i].GetDepth())
			continue
		}

		guard <- struct{}{}

		wg.Add(1)
		go func(item *models.Item) {
			defer wg.Done()
			defer func() { <-guard }()
			defer stats.URLsCrawledIncr()

			var (
				err  error
				resp *http.Response
			)

			// Execute the request
			req := item.GetURL().GetRequest()
			if req == nil {
				panic("request is nil")
			}
			if config.Get().Proxy != "" {
				resp, err = globalArchiver.ClientWithProxy.Do(req)
			} else {
				resp, err = globalArchiver.Client.Do(req)
			}
			if err != nil {
				logger.Error("unable to execute request", "err", err.Error(), "seed_id", seed.GetShortID(), "item_id", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops())
				item.SetStatus(models.ItemFailed)
				return
			}

			// Set the response in the URL
			item.GetURL().SetResponse(resp)

			// Process the body
			err = ProcessBody(item.GetURL(), config.Get().DisableAssetsCapture, config.Get().DomainsCrawl, config.Get().MaxHops, config.Get().WARCTempDir)
			if err != nil {
				logger.Error("unable to process body", "err", err.Error(), "item_id", item.GetShortID(), "seed_id", seed.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops())
				item.SetStatus(models.ItemFailed)
				return
			}

			stats.HTTPReturnCodesIncr(strconv.Itoa(resp.StatusCode))

			logger.Info("url archived", "url", item.GetURL().String(), "seed_id", seed.GetShortID(), "item_id", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops(), "status", resp.StatusCode)

			item.SetStatus(models.ItemArchived)
		}(items[i])
	}

	// Wait for all goroutines to finish
	wg.Wait()
}
