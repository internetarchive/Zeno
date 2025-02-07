package watchers

import (
	"context"
	"sync"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/archiver"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
)

var (
	wwqCtx, wwqCancel = context.WithCancel(context.Background())
	wwqWg             sync.WaitGroup
)

// WatchWARCWritingQueue watches the WARC writing queue size and pauses the pipeline if it exceeds the worker count
func WatchWARCWritingQueue(interval time.Duration) {
	wwqWg.Add(1)
	defer wwqWg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "controler.warcWritingQueueWatcher",
	})

	paused := false
	returnASAP := false
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-wwqCtx.Done():
			defer logger.Debug("closed")
			if paused {
				logger.Info("returning after resume")
				returnASAP = true
			}
			return
		case <-ticker.C:
			queueSize := archiver.GetWARCWritingQueueSize()

			logger.Debug("checking queue size", "queue_size", queueSize, "max_queue_size", config.Get().WorkersCount, "paused", paused)

			if queueSize > config.Get().WorkersCount && !paused {
				logger.Warn("WARC writing queue exceeded the worker count, pausing the pipeline")
				pause.Pause("WARC writing queue exceeded the worker count")
				paused = true
			} else if queueSize < config.Get().WorkersCount && paused {
				logger.Info("WARC writing queue size returned to acceptable, resuming the pipeline")
				pause.Resume()
				paused = false
				if returnASAP {
					return
				}
			}
		}
	}
}

// StopWARCWritingQueueWatcher stops the WARC writing queue watcher by canceling the context and waiting for the goroutine to finish
func StopWARCWritingQueueWatcher() {
	wwqCancel()
	wwqWg.Wait()
}
