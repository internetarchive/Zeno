package hq

import (
	"sync"

	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gocrawlhq"
)

func consumer() {
	var wg sync.WaitGroup // WaitGroup to track batch-sending goroutines

	for {
		select {
		case <-globalHQ.ctx.Done():
			// Received signal to stop
			// Wait for all batch-sending goroutines to finish
			wg.Wait()
			return
		default:
			// This is purposely evaluated every time,
			// because the value of workers might change
			// during the crawl in the future (to be implemented)
			var HQBatchSize = config.Get().WorkersCount

			// If a specific HQ batch size is set, use it
			if config.Get().HQBatchSize != 0 {
				HQBatchSize = config.Get().HQBatchSize
			}

			// Get a batch of URLs from crawl HQ
			URLs, err := getURLs(HQBatchSize)
			if err != nil {
				logger.Error("error getting new URLs from crawl HQ", "err", err.Error(), "func", "hq.Consumer")
				continue
			}

			// Channel to receive pre-fetch signal
			prefetchSignal := make(chan struct{}, 1)

			// Increment the WaitGroup counter
			wg.Add(1)

			// Send the URLs to the reactor in a goroutine
			go func(URLs []gocrawlhq.URL) {
				defer wg.Done() // Decrement the WaitGroup counter when done

				totalURLs := len(URLs)
				for i, URL := range URLs {
					UUID := uuid.New()
					newItem := &models.Item{
						UUID: &UUID,
						URL: &models.URL{
							Raw:  URL.Value,
							Hops: pathToHops(URL.Path),
						},
						Status: models.ItemFresh,
						Source: models.ItemSourceHQ,
					}

					if err := reactor.ReceiveInsert(newItem); err != nil {
						panic("couldn't insert seed in reactor")
					}

					// When one-third of the URLs are left, send a pre-fetch signal
					if i == totalURLs-totalURLs/3 {
						// Send pre-fetch signal to Consumer
						select {
						case prefetchSignal <- struct{}{}:
						default:
							// Signal already sent; do nothing
						}
					}

					// Check if stop signal is received to exit early
					select {
					case <-globalHQ.ctx.Done():
						// Stop signal received, exit the goroutine
						return
					default:
						// Continue sending URLs
					}
				}
			}(URLs)

			// Wait for pre-fetch signal or stop signal
			select {
			case <-prefetchSignal:
				// Received pre-fetch signal; continue to fetch next batch
				continue
			case <-globalHQ.ctx.Done():
				// Received signal to stop
				// Wait for all batch-sending goroutines to finish
				wg.Wait()
				return
			}
		}
	}
}

func getURLs(HQBatchSize int) ([]gocrawlhq.URL, error) {
	if config.Get().HQBatchConcurrency == 1 {
		return globalHQ.client.Get(HQBatchSize, config.Get().HQStrategy)
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	batchSize := HQBatchSize / config.Get().HQBatchConcurrency
	URLsChan := make(chan []gocrawlhq.URL, config.Get().HQBatchConcurrency)
	var URLs []gocrawlhq.URL

	// Start goroutines to get URLs from crawl HQ, each will request
	// HQBatchSize / HQConcurrentBatch URLs
	for i := 0; i < config.Get().HQBatchConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			URLs, err := globalHQ.client.Get(batchSize, config.Get().HQStrategy)
			if err != nil {
				logger.Error("error getting new URLs from crawl HQ", "err", err.Error(), "func", "hq.getURLs")
				return
			}
			URLsChan <- URLs
		}()
	}

	// Wait for all goroutines to finish
	go func() {
		wg.Wait()
		close(URLsChan)
	}()

	// Collect all URLs from the channels
	for URLsFromChan := range URLsChan {
		mu.Lock()
		URLs = append(URLs, URLsFromChan...)
		mu.Unlock()
	}

	return URLs, nil
}
