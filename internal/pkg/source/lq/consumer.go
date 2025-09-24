package lq

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/internal/pkg/source"
	"github.com/internetarchive/Zeno/internal/pkg/source/lq/sqlc_model"
	"github.com/internetarchive/Zeno/pkg/models"
)

var crawlFinishedOnce sync.Once

func (s *LQ) consumer() {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "lq.consumer",
	})

	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	// Set the batch size for fetching URLs
	batchSize := config.Get().WorkersCount

	// Create a fixed-size buffer (channel) for URLs
	urlBuffer := make(chan *sqlc_model.Url, batchSize)

	// WaitGroup to wait for goroutines to finish on shutdown
	var wg sync.WaitGroup

	// Start the consumerFetcher goroutine(s)
	wg.Add(1)
	go s.consumerFetcher(ctx, &wg, urlBuffer, batchSize)

	// Start the consumerSender goroutine(s)
	wg.Add(1)
	go s.consumerSender(ctx, &wg, urlBuffer)

	// Wait for shutdown signal
	for {
		select {
		case <-ctx.Done():
			logger.Debug("received done signal")

			// Cancel the context to stop all goroutines.
			cancel()

			logger.Debug("waiting for goroutines to finish")

			// Wait for all goroutines to finish
			wg.Wait()

			// Close the urlBuffer to signal consumerSenders to finish
			close(urlBuffer)

			s.wg.Done()

			logger.Debug("closed")
			return
		}
	}
}

func (s *LQ) consumerFetcher(ctx context.Context, wg *sync.WaitGroup, urlBuffer chan<- *sqlc_model.Url, batchSize int) {
	defer wg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "lq.consumerFetcher",
	})

	r := source.NewFeedEmptyReporter(logger)
	emptyFetches := 0

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			logger.Debug("closed")
			return
		default:
		}

		// Fetch URLs from LQ
		URLs, err := s.getURLs(batchSize)
		if err != nil {
			logger.Error("error fetching URLs from LQ", "err", err.Error(), "func", "lq.consumerFetcher")
		}

		if len(URLs) == 0 {
			emptyFetches++
			time.Sleep(250 * time.Millisecond)
			// Check if crawl is finished when queue is empty
			checkIfCrawlFinished(logger, emptyFetches)
		} else {
			emptyFetches = 0 // Reset counter when URLs are found
		}

		r.Report(len(URLs))

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

func (s *LQ) consumerSender(ctx context.Context, wg *sync.WaitGroup, urlBuffer <-chan *sqlc_model.Url) {
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
			parsedURL, err := models.NewURL(URL.Value)
			if err != nil {
				discard = true
			}
			parsedURL.SetHops(int(URL.Hops))
			newItem := models.NewItemWithID(URL.ID, &parsedURL, URL.Via)
			newItem.SetSource(models.ItemSourceQueue)

			if discard {
				logger.Debug("parsing failed, sending the item to finisher", "url", URL.Value)
				s.finishCh <- newItem
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

func (s *LQ) getURLs(batchSize int) ([]sqlc_model.Url, error) {
	return s.client.get(s.ctx, batchSize)
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

// checkIfCrawlFinished checks if the crawl is complete by verifying both:
// 1. Local queue has no fresh URLs (indicated by empty URLs slice)
// 2. Reactor has no active work in progress
// If both conditions are met, it triggers a graceful shutdown.
func checkIfCrawlFinished(logger *log.FieldedLogger, emptyFetches int) {
	// Only check after multiple consecutive empty fetches to avoid premature shutdown
	if emptyFetches < 5 {
		return
	}

	// Check if reactor has any active work
	reactorState := reactor.GetStateTable()
	if len(reactorState) == 0 {
		crawlFinishedOnce.Do(func() {
			logger.Info("crawl finished: no URLs in queue and no active work in reactor, triggering graceful shutdown")
			// Use clean exit to avoid race conditions in signal handler
			// Use testing.Testing() for more reliable test environment detection
			go func() {
				// Give a brief moment for the log message to be written
				time.Sleep(50 * time.Millisecond)
				if !testing.Testing() {
					// In production, exit cleanly
					// Note: We can't use controler.Stop() due to circular import restrictions
					os.Exit(0)
				}
				// In test environment, just return - the e2e test will detect completion via logs
			}()
		})
	} else {
		logger.Debug("reactor still has active work", "active_items", len(reactorState))
	}
}


