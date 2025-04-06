package lq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/internal/pkg/source/lq/sqlc_model"
	"github.com/internetarchive/Zeno/pkg/models"
)

func consumer() {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "lq.consumer",
	})

	ctx, cancel := context.WithCancel(globalLQ.ctx)
	defer cancel()

	// Set the batch size for fetching URLs
	batchSize := config.Get().WorkersCount

	// Create a fixed-size buffer (channel) for URLs
	urlBuffer := make(chan *sqlc_model.Url, batchSize)

	// WaitGroup to wait for goroutines to finish on shutdown
	var wg sync.WaitGroup

	// Start the consumerFetcher goroutine(s)
	wg.Add(1)
	go consumerFetcher(ctx, &wg, urlBuffer, batchSize)

	// Start the consumerSender goroutine(s)
	wg.Add(1)
	go consumerSender(ctx, &wg, urlBuffer)
	// Wait for shutdown signal
	for {
		select {
		case <-globalLQ.ctx.Done():
			logger.Debug("received done signal")

			// Cancel the context to stop all goroutines.
			cancel()

			logger.Debug("waiting for goroutines to finish")

			// Wait for all goroutines to finish
			wg.Wait()

			// Close the urlBuffer to signal consumerSenders to finish
			close(urlBuffer)

			globalLQ.wg.Done()

			logger.Debug("closed")
			return
		}
	}
}

func consumerFetcher(ctx context.Context, wg *sync.WaitGroup, urlBuffer chan<- *sqlc_model.Url, batchSize int) {
	defer wg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "lq.consumerFetcher",
	})

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			logger.Debug("closed")
			return
		default:
		}

		// Fetch URLs from LQ
		URLs, err := getURLs(batchSize)
		if err != nil || len(URLs) == 0 {
			if err != nil {
				logger.Error("error fetching URLs from LQ", "err", err.Error(), "func", "lq.consumerFetcher")
			} else {
				logger.Debug("feed is empty, waiting for new URLs")
			}
			time.Sleep(250 * time.Millisecond)
			continue
		}

		err = ensureAllIDsNotInReactor(URLs)
		if err != nil {
			spew.Dump(URLs)
			panic(err)
		}

		// Enqueue URLs into the buffer
		for i := range URLs {
			select {
			case <-ctx.Done():
				logger.Debug("closed")
				return
			case urlBuffer <- &sqlc_model.Url{
				ID:        URLs[i].ID,
				Value:     URLs[i].Value,
				Via:       URLs[i].Via,
				Hops:      URLs[i].Hops,
				Status:    URLs[i].Status,
				Timestamp: URLs[i].Timestamp,
			}: //Deep copy of the URL to ensure pointer alisaing does not cause issues
			}
		}

		// Empty the URL slice
		URLs = nil
	}
}

func consumerSender(ctx context.Context, wg *sync.WaitGroup, urlBuffer <-chan *sqlc_model.Url) {
	defer wg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "lq.consumerSender",
	})

	var previousURLReceived *sqlc_model.Url

	for {
		select {
		case <-ctx.Done():
			logger.Debug("closed")
			return
		case URL := <-urlBuffer:
			// Debug check to troubleshoot a problem where the same seed is received twice by the reactor
			if previousURLReceived != nil && previousURLReceived.ID == URL.ID {
				spew.Dump(previousURLReceived)
				spew.Dump(URL)
				panic("same seed received twice by lq.consumerSender")
			}
			urlCopy := *URL
			previousURLReceived = &urlCopy

			var discard bool
			// Process the URL and create a new Item
			parsedURL := models.URL{
				Raw:  URL.Value,
				Hops: int(URL.Hops),
			}
			err := parsedURL.Parse()
			if err != nil {
				discard = true
			}
			newItem := models.NewItem(URL.ID, &parsedURL, URL.Via)
			newItem.SetStatus(models.ItemFresh)
			newItem.SetSource(models.ItemSourceQueue)

			if discard {
				logger.Debug("parsing failed, sending the item to finisher", "url", URL.Value)
				globalLQ.finishCh <- newItem
				break
			}

			logger.Debug("sending new item to reactor", "item", newItem.GetShortID())

			// Send the new Item to the reactor
			err = reactor.ReceiveInsert(newItem)
			if err != nil {
				if err == reactor.ErrReactorFrozen {
					select {
					case <-ctx.Done():
						logger.Debug("closed while sending to frozen reactor")
						return
					}
				}
				panic(err)
			}
		}
	}
}

func getURLs(batchSize int) ([]sqlc_model.Url, error) {
	return globalLQ.client.Get(context.TODO(), batchSize)
}

func ensureAllIDsNotInReactor(URLs []sqlc_model.Url) error {
	reactorIDs := reactor.GetStateTable()
	reactorIDMap := make(map[string]struct{})
	for i := range reactorIDs {
		reactorIDMap[reactorIDs[i]] = struct{}{}
	}

	for i := range URLs {
		if _, ok := reactorIDMap[URLs[i].ID]; ok {
			return fmt.Errorf("URL ID %s found in reactor", URLs[i].ID)
		}
	}
	return nil
}
