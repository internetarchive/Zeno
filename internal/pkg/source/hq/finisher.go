package hq

import (
	"context"
	"sync"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/gocrawlhq"
)

type finishBatch struct {
	URLs           []gocrawlhq.URL
	ChildsCaptured int
}

// finisher initializes and starts the finisher and dispatcher processes.
func finisher() {
	// Create a context to manage goroutines
	ctx, cancel := context.WithCancel(globalHQ.ctx)
	defer cancel()

	maxSenders := getMaxFinishSenders()
	batchCh := make(chan *finishBatch, maxSenders)

	var wg sync.WaitGroup

	wg.Add(1)
	go finishReceiver(ctx, &wg, batchCh)

	wg.Add(1)
	go finishDispatcher(ctx, &wg, batchCh)

	// Wait for the context to be canceled.
	<-ctx.Done()

	// Cancel the context to stop all goroutines.
	cancel()

	// Wait for the finisher and dispatcher to finish.
	wg.Wait()
}

// finishReceiver reads URLs from finishCh, accumulates them into batches, and sends the batches to batchCh.
func finishReceiver(ctx context.Context, wg *sync.WaitGroup, batchCh chan *finishBatch) {
	defer wg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "hq.finishReceiver",
	})

	batchSize := getBatchSize()
	maxWaitTime := 5 * time.Second

	batch := &finishBatch{
		URLs: make([]gocrawlhq.URL, 0, batchSize),
	}
	timer := time.NewTimer(maxWaitTime)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			// Send any remaining URLs.
			if len(batch.URLs) > 0 {
				logger.Debug("while closing sending remaining batch to dispatcher", "size", len(batch.URLs))
				batchCh <- batch // Blocks if batchCh is full.
			}
			return
		case url := <-globalHQ.finishCh:
			URLToSend := gocrawlhq.URL{
				ID: url.ID,
			}
			batch.URLs = append(batch.URLs, URLToSend)
			if len(batch.URLs) >= batchSize {
				logger.Debug("sending batch to dispatcher", "size", len(batch.URLs))
				// Send the batch to batchCh.
				batchCh <- batch // Blocks if batchCh is full.
				batch.URLs = make([]gocrawlhq.URL, 0, batchSize)
				resetTimer(timer, maxWaitTime)
			}
		case <-timer.C:
			if len(batch.URLs) > 0 {
				logger.Debug("sending non-full batch to dispatcher", "size", len(batch.URLs))
				batchCh <- batch // Blocks if batchCh is full.
				batch.URLs = make([]gocrawlhq.URL, 0, batchSize)
			}
			resetTimer(timer, maxWaitTime)
		}
	}
}

// finishDispatcher receives batches from batchCh and dispatches them to sender routines.
func finishDispatcher(ctx context.Context, wg *sync.WaitGroup, batchCh chan *finishBatch) {
	defer wg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "hq.finishDispatcher",
	})

	maxSenders := getMaxFinishSenders()
	senderSemaphore := make(chan struct{}, maxSenders)
	var senderWg sync.WaitGroup

	for {
		select {
		case batch := <-batchCh:
			senderSemaphore <- struct{}{} // Blocks if maxSenders reached.
			senderWg.Add(1)
			logger.Debug("dispatching batch to sender", "size", len(batch.URLs))
			go func(batch *finishBatch) {
				defer senderWg.Done()
				defer func() { <-senderSemaphore }()
				finishSender(ctx, batch)
			}(batch)
		case <-ctx.Done():
			// Wait for all sender routines to finish.
			senderWg.Wait()
			return
		}
	}
}

// finishSender sends a batch of URLs to HQ with retries and exponential backoff.
func finishSender(ctx context.Context, batch *finishBatch) {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "hq.finishSender",
	})

	backoff := time.Second
	maxBackoff := 5 * time.Second

	for {
		err := globalHQ.client.Delete(batch.URLs, batch.ChildsCaptured)
		select {
		case <-ctx.Done():
			return
		default:
			if err != nil {
				logger.Error("Error sending batch to HQ", "err", err)
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

// resetTimer safely resets the timer to the specified duration.
func resetTimer(timer *time.Timer, duration time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(duration)
}

// getMaxFinishSenders returns the maximum number of sender routines based on configuration.
func getMaxFinishSenders() int {
	workersCount := config.Get().WorkersCount
	if workersCount < 10 {
		return 1
	}
	return workersCount / 10
}

// getBatchSize returns the batch size based on configuration.
func getBatchSize() int {
	batchSize := config.Get().WorkersCount
	if batchSize == 0 {
		batchSize = 100 // Default batch size.
	}
	return batchSize
}
