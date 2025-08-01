package hq

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/internal/pkg/source"
	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gocrawlhq"
	"golang.org/x/sync/errgroup"
)

func (s *HQ) consumer() {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "hq.consumer",
	})

	// Create a context to manage goroutines
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	// Set the batch size for fetching URLs
	batchSize := config.Get().HQBatchSize

	// Create a fixed-size buffer (channel) for URLs
	urlBuffer := make(chan *gocrawlhq.URL, batchSize)

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
		case <-s.ctx.Done():
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

func (s *HQ) consumerFetcher(ctx context.Context, wg *sync.WaitGroup, urlBuffer chan<- *gocrawlhq.URL, batchSize int) {
	defer wg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "hq.consumerFetcher",
	})

	r := source.NewFeedEmptyReporter(logger)

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			logger.Debug("closed")
			return
		default:
		}

		// Fetch URLs from HQ
		URLs, err := s.getURLs(batchSize)
		if err != nil {
			logger.Error("error fetching URLs from CrawlHQ", "err", err.Error(), "func", "hq.consumerFetcher")
		}

		if len(URLs) == 0 {
			time.Sleep(500 * time.Millisecond)
		}

		r.Report(len(URLs))

		err = ensureAllURLsUnique(URLs)
		if err != nil {
			spew.Dump(URLs)
			panic(err)
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
			case urlBuffer <- &gocrawlhq.URL{
				ID:        URLs[i].ID,
				Value:     URLs[i].Value,
				Via:       URLs[i].Via,
				Host:      URLs[i].Host,
				Path:      URLs[i].Path,
				Type:      URLs[i].Type,
				Crawler:   URLs[i].Crawler,
				Status:    URLs[i].Status,
				LiftOff:   URLs[i].LiftOff,
				Timestamp: URLs[i].Timestamp,
			}: //Deep copy of the URL to ensure pointer alisaing does not cause issues
			}
		}

		// Empty the URL slice
		URLs = nil
	}
}

func (s *HQ) consumerSender(ctx context.Context, wg *sync.WaitGroup, urlBuffer <-chan *gocrawlhq.URL) {
	defer wg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "hq.consumerSender",
	})

	var previousURLReceived *gocrawlhq.URL

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
				panic("same seed received twice by hq.consumerSender")
			}
			urlCopy := *URL
			previousURLReceived = &urlCopy

			var discard bool
			// Process the URL and create a new Item
			parsedURL, err := models.NewURL(URL.Value)
			if err != nil {
				discard = true
			}
			parsedURL.SetHops(pathToHops(URL.Path))
			newItem := models.NewItemWithID(URL.ID, &parsedURL, URL.Via)
			newItem.SetStatus(models.ItemFresh)
			newItem.SetSource(models.ItemSourceHQ)

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

// getURLs fetch URLs from CrawlHQ with optional concurrency.
//
// If HQBatchConcurrency > 1, all URLs fetched will be returned (EVEN IF some requests fail) but
// only the first error will be returned.
func (s *HQ) getURLs(batchSize int) ([]gocrawlhq.URL, error) {
	if config.Get().HQBatchConcurrency <= 1 {
		return s.client.Get(context.TODO(), batchSize)
	}

	concurrency := config.Get().HQBatchConcurrency
	subBatchSize := batchSize / concurrency
	urlsChan := make(chan []gocrawlhq.URL)
	var allURLs []gocrawlhq.URL

	g, _ := errgroup.WithContext(context.TODO())

	// Start concurrent fetches
	for range concurrency {
		g.Go(func() error {
			// Here we use a new context instead of errorgroup context:
			// We don't want to cancel other fetches if one fails, that may
			// lead to dropping URLs fetched midway through HTTP.
			URLs, err := s.client.Get(context.TODO(), subBatchSize)
			if err != nil {
				return err
			}
			urlsChan <- URLs
			return nil
		})
	}

	go func() {
		g.Wait()
		close(urlsChan)
	}()

	// Collect URLs from all fetches
	for URLs := range urlsChan {
		if len(URLs) != 0 {
			allURLs = append(allURLs, URLs...)
		}
	}

	return allURLs, g.Wait()
}

func ensureAllURLsUnique(URLs []gocrawlhq.URL) error {
	seen := make(map[string]struct{})
	for _, URL := range URLs {
		if _, ok := seen[URL.ID]; ok {
			return errors.New("duplicate URL ID found")
		}
		seen[URL.ID] = struct{}{}
	}
	return nil
}

func ensureAllIDsNotInReactor(URLs []gocrawlhq.URL) error {
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
