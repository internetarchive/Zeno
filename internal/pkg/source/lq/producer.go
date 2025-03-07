package lq

import (
	"context"
	"sync"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/source/lq/sqlc_model"
)

// producerBatch represents a batch of URLs to be added to LQ.
type producerBatch struct {
	URLs []sqlc_model.Url
}

// sqlite only accepts one write at a time, so hardcoding this to 2
// allows one sender operation to be in progress while another is being prepared/blocking
const maxSenders = 2

func producer() {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "lq.producer",
	})

	// Create a context to manage goroutines
	ctx, cancel := context.WithCancel(globalLQ.ctx)
	defer cancel()

	batchCh := make(chan *producerBatch, maxSenders)

	var wg sync.WaitGroup

	wg.Add(1)
	go producerReceiver(ctx, &wg, batchCh)

	wg.Add(1)
	go producerDispatcher(ctx, &wg, batchCh)

	// Wait for the context to be canceled.
	for {
		select {
		case <-globalLQ.ctx.Done():
			logger.Debug("received done signal")

			// Cancel the context to stop all goroutines.
			cancel()

			logger.Debug("waiting for goroutines to finish")

			// Wait for the producer and dispatcher to finish.
			wg.Wait()

			// Close the batch channel to signal the dispatcher to finish.
			close(batchCh)

			globalLQ.wg.Done()

			logger.Debug("closed")
			return
		}
	}
}

// producerReceiver reads URLs from produceCh, accumulates them into batches, and sends the batches to batchCh.
func producerReceiver(ctx context.Context, wg *sync.WaitGroup, batchCh chan *producerBatch) {
	defer wg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "lq.producerReceiver",
	})

	batchSize := 100
	maxWaitTime := 5 * time.Second

	batch := &producerBatch{
		URLs: make([]sqlc_model.Url, 0, batchSize),
	}
	timer := time.NewTimer(maxWaitTime)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Debug("closing")
			return
		case item := <-globalLQ.produceCh:
			URL := sqlc_model.Url{
				Value: item.GetURL().Raw,
				Via:   item.GetSeedVia(),
				Hops:  int64(item.GetURL().GetHops()),
			}
			batch.URLs = append(batch.URLs, URL)
			if len(batch.URLs) >= batchSize {
				logger.Debug("sending batch to dispatcher", "size", len(batch.URLs))
				// Send the batch to batchCh.
				copyBatch := *batch
				select {
				case <-ctx.Done():
					logger.Debug("closed")
					return
				case batchCh <- &copyBatch: // Blocks if batchCh is full.
				}
				batch = &producerBatch{
					URLs: make([]sqlc_model.Url, 0, batchSize),
				}
				resetTimer(timer, maxWaitTime)
			}
		case <-timer.C:
			if len(batch.URLs) > 0 {
				logger.Debug("sending non-full batch to dispatcher", "size", len(batch.URLs))
				copyBatch := *batch
				select {
				case <-ctx.Done():
					logger.Debug("closed")
					return
				case batchCh <- &copyBatch: // Blocks if batchCh is full.
				}
				batch = &producerBatch{
					URLs: make([]sqlc_model.Url, 0, batchSize),
				}
			}
			resetTimer(timer, maxWaitTime)
		}
	}
}

// producerDispatcher receives batches from batchCh and dispatches them to sender routines.
func producerDispatcher(ctx context.Context, wg *sync.WaitGroup, batchCh chan *producerBatch) {
	defer wg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "lq.producerDispatcher",
	})

	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-batchCh:
			logger.Debug("dispatching batch to sender", "size", len(batch.URLs))
			if err := globalLQ.client.Add(ctx, batch.URLs, false); err != nil {
				logger.Error("failed to send batch to LQ", "error", err)
			}
		}
	}
}
