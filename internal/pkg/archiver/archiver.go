package archiver

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/CorentinB/warc"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/internetarchive/Zeno/pkg/models"
)

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

					if item.GetStatus() == models.ItemFailed || item.GetStatus() == models.ItemCompleted {
						logger.Debug("skipping item", "item", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops(), "status", item.GetStatus().String())
					} else {
						err := item.CheckConsistency()
						if err != nil {
							panic(err)
						}
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

			// Consume the response body
			body := bytes.NewBuffer(nil)
			_, err = io.Copy(body, resp.Body)
			if err != nil {
				logger.Error("unable to read response body", "err", err.Error(), "seed_id", seed.GetShortID(), "item_id", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops())
				item.SetStatus(models.ItemFailed)
				return
			}

			// Set the body in the URL
			item.GetURL().SetBody(bytes.NewReader(body.Bytes()))

			stats.HTTPReturnCodesIncr(strconv.Itoa(resp.StatusCode))

			logger.Info("url archived", "url", item.GetURL().String(), "seed_id", seed.GetShortID(), "item_id", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops())

			item.SetStatus(models.ItemArchived)
		}(items[i])
	}

	// Wait for all goroutines to finish
	wg.Wait()
}
