package lq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/source/lq/sqlc_model"
	"github.com/internetarchive/Zeno/pkg/models"
)

type finishBatch struct {
	URLs           []sqlc_model.Url
	ChildsCaptured int
}

// finisher initializes and starts the finisher and dispatcher processes.
func finisher() {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": ".q.finisher",
	})

	// Create a context to manage goroutines
	ctx, cancel := context.WithCancel(globalLQ.ctx)
	defer cancel()

	maxSenders := getMaxFinishSenders()
	batchCh := make(chan *finishBatch, maxSenders)

	var wg sync.WaitGroup

	wg.Add(1)
	go finisherReceiver(ctx, &wg, batchCh)

	wg.Add(1)
	go finisherDispatcher(ctx, &wg, batchCh)

	// Wait for the context to be canceled.
	for {
		select {
		case <-globalLQ.ctx.Done():
			logger.Debug("received done signal")

			// Cancel the context to stop all goroutines.
			cancel()

			logger.Debug("waiting for goroutines to finish")

			// Wait for the finisher and dispatcher to finish.
			wg.Wait()

			// Close the batch channel to signal the dispatcher to finish.
			close(batchCh)

			globalLQ.wg.Done()

			logger.Debug("closed")
			return
		}
	}
}

// finisherReceiver reads URLs from finishCh, accumulates them into batches, and sends the batches to batchCh.
func finisherReceiver(ctx context.Context, wg *sync.WaitGroup, batchCh chan *finishBatch) {
	defer wg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "lq.finisherReceiver",
	})

	batchSize := getBatchSize()
	maxWaitTime := 5 * time.Second

	batch := &finishBatch{
		URLs: make([]sqlc_model.Url, 0, batchSize),
	}
	timer := time.NewTimer(maxWaitTime)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Debug("closed")
			return
		case item := <-globalLQ.finishCh:
			logger.Debug("received item", "item", item.GetShortID())

			var value string
			// If preprocessing failed, there will be nil values here
			if item.GetURL() != nil && item.GetURL().GetParsed() != nil {
				value = item.GetURL().String()
			}

			URL := sqlc_model.Url{
				ID:    item.GetID(),
				Value: value,
			}

			batch.URLs = append(batch.URLs, URL)
			item.Traverse(func(itemTraversed *models.Item) {
				if itemTraversed.IsChild() {
					batch.ChildsCaptured++
				}
			})
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
				batch = &finishBatch{
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
				batch = &finishBatch{
					URLs: make([]sqlc_model.Url, 0, batchSize),
				}
			}
			resetTimer(timer, maxWaitTime)
		}
	}
}

// finisherDispatcher receives batches from batchCh and dispatches them to sender routines.
func finisherDispatcher(ctx context.Context, wg *sync.WaitGroup, batchCh chan *finishBatch) {
	defer wg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "lq.finisherDispatcher",
	})

	maxSenders := getMaxFinishSenders()
	senderSemaphore := make(chan struct{}, maxSenders)
	var senderWg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			logger.Debug("waiting for sender routines to finish")
			// Wait for all sender routines to finish.
			senderWg.Wait()
			logger.Debug("closed")
			return
		case batch := <-batchCh:
			batchUUID := uuid.NewString()[:6]
			senderSemaphore <- struct{}{} // Blocks if maxSenders reached.
			senderWg.Add(1)
			logger.Debug("dispatching batch to sender", "size", len(batch.URLs))
			go func(batch *finishBatch, batchUUID string) {
				defer senderWg.Done()
				defer func() { <-senderSemaphore }()
				finisherSender(ctx, batch, batchUUID)
			}(batch, batchUUID)
		}
	}
}

// finisherSender sends a batch of URLs to LQ with retries and exponential backoff.
func finisherSender(ctx context.Context, batch *finishBatch, batchUUID string) {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": fmt.Sprintf("lq.finisherSender.%s", batchUUID),
	})
	defer logger.Debug("done")

	backoff := time.Second
	maxBackoff := 5 * time.Second

	logger.Debug("sending batch to LQ", "size", len(batch.URLs))

	for {
		err := globalLQ.client.Delete(context.TODO(), batch.URLs, false)
		select {
		case <-ctx.Done():
			logger.Debug("closing")
			return
		default:
			if err != nil {
				logger.Error("error sending batch to LQ", "err", err)
				time.Sleep(backoff)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}
			return
		}
	}
}

// getMaxFinishSenders returns the maximum number of sender routines based on configuration.
func getMaxFinishSenders() int {
	return 2
}

// getBatchSize returns the batch size based on configuration.
func getBatchSize() int {
	batchSize := config.Get().WorkersCount
	if batchSize == 0 {
		batchSize = 100 // Default batch size.
	}
	return batchSize
}
