package hq

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gocrawlhq"
)

func consumer() {
	// Create a context to manage goroutines
	ctx, cancel := context.WithCancel(globalHQ.ctx)
	defer cancel()

	// Set the batch size for fetching URLs
	batchSize := config.Get().HQBatchSize

	// Create a fixed-size buffer (channel) for URLs
	urlBuffer := make(chan *gocrawlhq.URL, batchSize)

	// WaitGroup to wait for goroutines to finish on shutdown
	var wg sync.WaitGroup

	// Start the fetcher goroutine(s)
	wg.Add(1)
	go fetcher(ctx, &wg, urlBuffer, batchSize)

	// Start the sender goroutine(s)
	wg.Add(1)
	go sender(ctx, &wg, urlBuffer)

	// Wait for shutdown signal
	<-globalHQ.ctx.Done()

	// Cancel the context to stop all goroutines
	cancel()

	// Wait for all goroutines to finish
	wg.Wait()

	// Close the urlBuffer to signal senders to finish
	close(urlBuffer)
}

func fetcher(ctx context.Context, wg *sync.WaitGroup, urlBuffer chan<- *gocrawlhq.URL, batchSize int) {
	defer wg.Done()
	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Fetch URLs from HQ
		URLs, err := getURLs(batchSize)
		if err != nil {
			logger.Error("error fetching URLs from CrawlHQ", "err", err.Error(), "func", "hq.fetcher")
			time.Sleep(250 * time.Millisecond)
			continue
		}

		// Enqueue URLs into the buffer
		for _, URL := range URLs {
			select {
			case <-ctx.Done():
				return
			case urlBuffer <- &URL:

			}
		}
	}
}

func sender(ctx context.Context, wg *sync.WaitGroup, urlBuffer <-chan *gocrawlhq.URL) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case URL, ok := <-urlBuffer:
			if !ok {
				// Channel closed, exit the sender
				return
			}

			// Process the URL and send to reactor
			err := processAndSend(URL)
			if err != nil {
				panic(err)
			}
		}
	}
}

func processAndSend(URL *gocrawlhq.URL) error {
	UUID := uuid.New()
	newItem := &models.Item{
		UUID: &UUID,
		URL: &models.URL{
			Raw:  URL.Value,
			Hops: 0,
		},
		Status: models.ItemFresh,
		Source: models.ItemSourceHQ,
	}

	// Send the item to the reactor
	err := reactor.ReceiveInsert(newItem)
	if err != nil {
		return err
	}
	return nil
}

func getURLs(batchSize int) ([]gocrawlhq.URL, error) {
	// Fetch URLs from CrawlHQ with optional concurrency
	if config.Get().HQBatchConcurrency == 1 {
		return globalHQ.client.Get(batchSize, config.Get().HQStrategy)
	}

	var wg sync.WaitGroup
	concurrency := config.Get().HQBatchConcurrency
	subBatchSize := batchSize / concurrency
	urlsChan := make(chan []gocrawlhq.URL, concurrency)
	var allURLs []gocrawlhq.URL

	// Start concurrent fetches
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			URLs, err := globalHQ.client.Get(subBatchSize, config.Get().HQStrategy)
			if err != nil {
				logger.Error("error fetching URLs from CrawlHQ", "err", err.Error(), "func", "hq.getURLs")
				return
			}
			urlsChan <- URLs
		}()
	}

	// Wait for all fetches to complete
	wg.Wait()
	close(urlsChan)

	// Collect URLs from all fetches
	for URLs := range urlsChan {
		allURLs = append(allURLs, URLs...)
	}

	return allURLs, nil
}
