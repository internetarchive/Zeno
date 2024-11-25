package archiver

import (
	"bytes"
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

	for {
		select {
		// Closes the run routine when context is canceled
		case <-globalArchiver.ctx.Done():
			logger.Debug("shutting down")
			wg.Wait()
			return
		case item, ok := <-globalArchiver.inputCh:
			if ok {
				logger.Debug("received item", "item", item.GetShortID())
				guard <- struct{}{}
				wg.Add(1)
				stats.ArchiverRoutinesIncr()
				go func(ctx context.Context) {
					defer wg.Done()
					defer func() { <-guard }()
					defer stats.ArchiverRoutinesDecr()

					archive(item)

					select {
					case <-ctx.Done():
						return
					case globalArchiver.outputCh <- item:
					}
				}(ctx)
			}
		}
	}
}

func archive(item *models.Item) {
	// TODO: rate limiting handling
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "archiver.archive",
	})

	var (
		URLsToCapture []*models.URL
		guard         = make(chan struct{}, config.Get().MaxConcurrentAssets)
		wg            sync.WaitGroup
		itemState     = models.ItemCaptured
	)

	// Determine the URLs that need to be captured
	if item.GetRedirection() != nil {
		URLsToCapture = append(URLsToCapture, item.GetRedirection())
	} else if len(item.GetChildren()) > 0 {
		URLsToCapture = append(URLsToCapture, item.GetChildren()...)
	} else {
		URLsToCapture = append(URLsToCapture, item.GetURL())
	}

	// Create a channel to collect successful URLs
	successfulURLsChan := make(chan *models.URL, len(URLsToCapture))

	for _, URL := range URLsToCapture {
		guard <- struct{}{}
		wg.Add(1)
		go func(URL *models.URL) {
			defer wg.Done()
			defer func() { <-guard }()
			defer stats.URLsCrawledIncr()

			var (
				err  error
				resp *http.Response
			)

			// Execute the request
			if config.Get().Proxy != "" {
				resp, err = globalArchiver.ClientWithProxy.Do(URL.GetRequest())
			} else {
				resp, err = globalArchiver.Client.Do(URL.GetRequest())
			}
			if err != nil {
				logger.Error("unable to execute request", "err", err.Error(), "func", "archiver.archive")

				// Only mark the item as failed if processing a redirection or a new seed
				if item.GetStatus() == models.ItemFresh || item.GetRedirection() != nil {
					itemState = models.ItemFailed
				}

				// Do not send URL to successfulURLsChan, effectively removing it
				return
			}

			// Set the response in the URL
			URL.SetResponse(resp)

			// Consume the response body
			body := bytes.NewBuffer(nil)
			_, err = io.Copy(body, resp.Body)
			if err != nil {
				logger.Error("unable to read response body", "err", err.Error(), "item", item.GetShortID())
				// Do not send URL to successfulURLsChan, effectively removing it
				return
			}

			// Set the body in the URL
			URL.SetBody(bytes.NewReader(body.Bytes()))

			logger.Info("url archived", "url", URL.String(), "item", item.GetShortID(), "status", resp.StatusCode)

			// Send the successful URL to the channel
			successfulURLsChan <- URL

			// If the URL is a child UChildRL, increment the captured count
			if containsURL(item.GetChildren(), URL) {
				item.IncrChildrenCaptured()
			}
		}(URL)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Close the channel since all sends are done
	close(successfulURLsChan)

	// Collect successful URLs from the channel
	var successfulURLs []*models.URL
	for URL := range successfulURLsChan {
		successfulURLs = append(successfulURLs, URL)
	}

	// Update URLsToCapture to only include successful URLs
	URLsToCapture = successfulURLs

	// Update item.Children if necessary
	if len(item.GetChildren()) > 0 {
		var successfulChildren []*models.URL
		for _, child := range item.GetChildren() {
			if containsURL(successfulURLs, child) {
				successfulChildren = append(successfulChildren, child)
			}
		}
		item.SetChildren(successfulChildren)
	}

	item.SetStatus(itemState)
}

func containsURL(urls []*models.URL, target *models.URL) bool {
	for _, url := range urls {
		if url == target {
			return true
		}
	}
	return false
}
