package hq

// import (
// 	"context"
// 	"sync"
// 	"time"

// 	"github.com/internetarchive/Zeno/internal/pkg/config"
// 	"github.com/internetarchive/Zeno/internal/pkg/log"
// 	"github.com/internetarchive/gocrawlhq"
// )

// var (
// 	// batchCh is a buffered channel that holds batches ready to be sent to HQ.
// 	// Its capacity is set to the maximum number of sender routines.
// 	batchCh chan []*gocrawlhq.URL
// )

// // finisher initializes and starts the finisher and dispatcher processes.
// func finisher() {
// 	var wg sync.WaitGroup

// 	maxSenders := getMaxFinishSenders()
// 	batchCh = make(chan []*gocrawlhq.URL, maxSenders)

// 	wg.Add(1)
// 	go receiver(ctx, &wg)

// 	wg.Add(1)
// 	go dispatcher(ctx, &wg)

// 	// Wait for the context to be canceled.
// 	<-ctx.Done()

// 	// Wait for the finisher and dispatcher to finish.
// 	wg.Wait()
// }

// // finishReceiver reads URLs from finishCh, accumulates them into batches, and sends the batches to batchCh.
// func finishReceiver(ctx context.Context, wg *sync.WaitGroup) {
// 	defer wg.Done()

// 	logger := log.NewFieldedLogger(&log.Fields{
// 		"component": "hq/finishReceiver",
// 	})

// 	batchSize := getBatchSize()
// 	maxWaitTime := 5 * time.Second

// 	batch := make([]*gocrawlhq.URL, 0, batchSize)
// 	timer := time.NewTimer(maxWaitTime)
// 	defer timer.Stop()

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			// Send any remaining URLs.
// 			if len(batch) > 0 {
// 				batchCh <- batch // Blocks if batchCh is full.
// 			}
// 			return
// 		case url := <-globalHQ.finishCh:
// 			URLToSend := &gocrawlhq.URL{
// 				ID: url.ID,
// 			}
// 			batch = append(batch, &URLToSend)
// 			if len(batch) >= batchSize {
// 				// Send the batch to batchCh.
// 				batchCh <- batch // Blocks if batchCh is full.
// 				batch = make([]gocrawlhq.URL, 0, batchSize)
// 				resetTimer(timer, maxWaitTime)
// 			}
// 		case <-timer.C:
// 			if len(batch) > 0 {
// 				batchCh <- batch // Blocks if batchCh is full.
// 				batch = make([]gocrawlhq.URL, 0, batchSize)
// 			}
// 			resetTimer(timer, maxWaitTime)
// 		}
// 	}
// }

// // finishDispatcher receives batches from batchCh and dispatches them to sender routines.
// func finishDispatcher(ctx context.Context, wg *sync.WaitGroup) {
// 	defer wg.Done()

// 	logger := log.NewFieldedLogger(&log.Fields{
// 		"component": "hq/dispatcher",
// 	})

// 	maxSenders := getMaxFinishSenders()
// 	senderSemaphore := make(chan struct{}, maxSenders)
// 	var senderWg sync.WaitGroup

// 	for {
// 		select {
// 		case batch := <-batchCh:
// 			senderSemaphore <- struct{}{} // Blocks if maxSenders reached.
// 			senderWg.Add(1)
// 			go func(batch []gocrawlhq.URL) {
// 				defer senderWg.Done()
// 				defer func() { <-senderSemaphore }()
// 				finishSender(ctx, batch)
// 			}(batch)
// 		case <-ctx.Done():
// 			// Wait for all sender routines to finish.
// 			senderWg.Wait()
// 			return
// 		}
// 	}
// }

// // finishSender sends a batch of URLs to HQ with retries and exponential backoff.
// func finishSender(ctx context.Context, batch []gocrawlhq.URL) {
// 	logger := log.NewFieldedLogger(&log.Fields{
// 		"component": "hq/finishSender",
// 	})

// 	backoff := time.Second
// 	maxBackoff := 5 * time.Second

// 	for {
// 		err := globalHQ.client.Delete(batch)
// 		select {
// 		case <-ctx.Done():
// 			return
// 		default:
// 			if err != nil {
// 				logger.Error("Error sending batch to HQ", "err", err)
// 				time.Sleep(backoff)
// 				backoff *= 2
// 				if backoff > maxBackoff {
// 					backoff = maxBackoff
// 				}
// 				continue
// 			}
// 			return
// 		}
// 	}
// }

// // resetTimer safely resets the timer to the specified duration.
// func resetTimer(timer *time.Timer, duration time.Duration) {
// 	if !timer.Stop() {
// 		select {
// 		case <-timer.C:
// 		default:
// 		}
// 	}
// 	timer.Reset(duration)
// }

// // getMaxFinishSenders returns the maximum number of sender routines based on configuration.
// func getMaxFinishSenders() int {
// 	workersCount := config.Get().WorkersCount
// 	if workersCount < 10 {
// 		return 1
// 	}
// 	return workersCount / 10
// }

// // getBatchSize returns the batch size based on configuration.
// func getBatchSize() int {
// 	batchSize := config.Get().HQBatchSize
// 	if batchSize == 0 {
// 		batchSize = 100 // Default batch size.
// 	}
// 	return batchSize
// }
