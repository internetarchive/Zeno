package archiver

import (
	"context"
	"io"
	"net/http"
	"sync"

	"github.com/CorentinB/warc"
	"github.com/internetarchive/Zeno/internal/pkg/config"
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
	errorCh  chan *models.Item

	Client          *warc.CustomHTTPClient
	ClientWithProxy *warc.CustomHTTPClient
}

var (
	globalArchiver *archiver
	once           sync.Once
	logger         *log.FieldedLogger
)

// This functions starts the archiver responsible for capturing the URLs
func Start(inputChan, outputChan, errorChan chan *models.Item) error {
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
			errorCh:  errorChan,
		}

		// Setup WARC writing HTTP clients
		startWARCWriter()

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
		close(globalArchiver.outputCh)
		logger.Info("stopped")
	}
}

func run() {
	defer globalArchiver.wg.Done()

	var (
		wg    sync.WaitGroup
		guard = make(chan struct{}, config.Get().WorkersCount)
	)

	for {
		select {
		// Closes the run routine when context is canceled
		case <-globalArchiver.ctx.Done():
			logger.Info("shutting down")
			return
		case item, ok := <-globalArchiver.inputCh:
			if ok {
				logger.Info("received item", "item", item.UUID.String())
				guard <- struct{}{}
				wg.Add(1)
				stats.ArchiverRoutinesIncr()
				go func() {
					defer wg.Done()
					defer func() { <-guard }()
					defer stats.ArchiverRoutinesDecr()
					archive(item)
				}()
			}
		}
	}
}

func archive(item *models.Item) {
	// TODO: rate limiting handling

	var (
		URLsToCapture []*models.URL
		guard         = make(chan struct{}, config.Get().MaxConcurrentAssets)
		wg            sync.WaitGroup
	)

	// Determines the URLs that need to be captured, if the item's status is fresh we need
	// to capture the seed, else we need to capture the child URLs (assets), in parallel
	if item.Status == models.ItemFresh {
		URLsToCapture = append(URLsToCapture, item.URL)
	} else {
		URLsToCapture = item.Childs
	}

	for _, URL := range URLsToCapture {
		guard <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-guard }()

			var (
				err  error
				resp *http.Response
			)

			if config.Get().Proxy != "" {
				resp, err = globalArchiver.ClientWithProxy.Do(URL.GetRequest())
			} else {
				resp, err = globalArchiver.Client.Do(URL.GetRequest())
			}
			if err != nil {
				logger.Error("unable to execute request", "err", err.Error(), "func", "archiver.archive")
				return
			}

			if resp.StatusCode != 200 {
				logger.Warn("non-200 status code", "status_code", resp.StatusCode)
			}

			// For now, we only consume it
			_, err = io.Copy(io.Discard, resp.Body)
			if err != nil {
				logger.Error("unable to consume response body", "url", URL.String(), "err", err.Error(), "func", "archiver.archive")
			}
		}()
	}

	wg.Wait()

	globalArchiver.outputCh <- item
}
