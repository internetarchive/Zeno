package archiver

import (
	"context"
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

// Start initializes the internal archiver structure, start the WARC writer and start routines, should only be called once and returns an error if called more than once
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

		logger.Debug("WARC writer started")

		for i := 0; i < config.Get().WorkersCount; i++ {
			globalArchiver.wg.Add(1)
			go globalArchiver.worker(strconv.Itoa(i))
		}

		logger.Info("started")
		done = true
	})

	if !done {
		return ErrArchiverAlreadyInitialized
	}

	return nil
}

// Stop stops the archiver routines and the WARC writer
func Stop() {
	if globalArchiver != nil {
		globalArchiver.cancel()
		globalArchiver.wg.Wait()

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

func (a *archiver) worker(workerID string) {
	defer a.wg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "archiver.worker",
		"worker_id": workerID,
	})

	defer logger.Debug("worker stopped")

	// Subscribe to the pause controler
	controlChans := pause.Subscribe()
	defer pause.Unsubscribe(controlChans)

	stats.ArchiverRoutinesIncr()
	defer stats.ArchiverRoutinesDecr()

	for {
		select {
		case <-a.ctx.Done():
			logger.Debug("shutting down")
			return
		case <-controlChans.PauseCh:
			logger.Debug("received pause event")
			controlChans.ResumeCh <- struct{}{}
			logger.Debug("received resume event")
		case seed, ok := <-a.inputCh:
			if ok {
				logger.Debug("received seed", "seed", seed.GetShortID(), "depth", seed.GetDepth(), "hops", seed.GetURL().GetHops())

				if err := seed.CheckConsistency(); err != nil {
					panic(fmt.Sprintf("seed consistency check failed with err: %s, seed id %s", err.Error(), seed.GetShortID()))
				}

				if seed.GetStatus() != models.ItemPreProcessed && seed.GetStatus() != models.ItemGotRedirected && seed.GetStatus() != models.ItemGotChildren {
					logger.Debug("skipping seed", "seed", seed.GetShortID(), "depth", seed.GetDepth(), "hops", seed.GetURL().GetHops(), "status", seed.GetStatus().String())
				} else {
					archive(workerID, seed)
				}

				select {
				case <-a.ctx.Done():
					logger.Debug("aborting seed due to stop", "seed", seed.GetShortID(), "depth", seed.GetDepth(), "hops", seed.GetURL().GetHops())
					return
				case a.outputCh <- seed:
				}
			}
		}
	}
}

func archive(workerID string, seed *models.Item) {
	// TODO: rate limiting handling
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "archiver.archive",
		"worker_id": workerID,
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

			// Get and measure request time
			getStartTime := time.Now()
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
			stats.MeanHTTPRespTimeAdd(time.Since(getStartTime))

			// Set the response in the URL
			item.GetURL().SetResponse(resp)

			// Process the body and measure the time
			processStartTime := time.Now()
			err = ProcessBody(item.GetURL(), config.Get().DisableAssetsCapture, domainscrawl.Enabled(), config.Get().MaxHops, config.Get().WARCTempDir)
			if err != nil {
				logger.Error("unable to process body", "err", err.Error(), "item_id", item.GetShortID(), "seed_id", seed.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops())
				item.SetStatus(models.ItemFailed)
				return
			}
			stats.MeanProcessBodyTimeAdd(uint64(time.Since(processStartTime)))

			stats.HTTPReturnCodesIncr(strconv.Itoa(resp.StatusCode))

			logger.Info("url archived", "url", item.GetURL().String(), "seed_id", seed.GetShortID(), "item_id", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops(), "status", resp.StatusCode)

			item.SetStatus(models.ItemArchived)
		}(items[i])
	}

	// Wait for all goroutines to finish
	wg.Wait()

	return
}
