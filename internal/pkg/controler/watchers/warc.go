package watchers

import (
	"context"
	"sync"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/archiver"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
)

var (
	wwqCtx, wwqCancel = context.WithCancel(context.Background())
	wwqWg             sync.WaitGroup
)

// StartWatchWARCWritingQueue watches the WARC writing queue size and pauses the pipeline if it exceeds the worker count
func StartWatchWARCWritingQueue(pauseCheckInterval time.Duration, pauseTimeout time.Duration, statsUpdateInterval time.Duration) {
	// If Zeno writes WARCs synchronously, no need to check for the queue size and pause the pipeline
	if config.Get().WARCWriteAsync {
		// Watch the WARC writing queue size and pause the pipeline if it exceeds the worker count
		wwqWg.Add(1)
		go func() {
			defer wwqWg.Done()

			logger := log.NewFieldedLogger(&log.Fields{
				"component": "controler.warcWritingQueueWatcher.pause",
			})
			defer logger.Debug("closed")

			var lastPauseTime time.Time
			paused := false
			returnAfterResume := false

			pauseTicker := time.NewTicker(pauseCheckInterval)
			defer pauseTicker.Stop()

			maxQueueSize := config.Get().WARCQueueSize
			if maxQueueSize == -1 || maxQueueSize == 0 {
				maxQueueSize = config.Get().WARCPoolSize
			}

			for {
				select {
				case <-wwqCtx.Done():
					if paused && !returnAfterResume {
						logger.Info("returning after resume")
						returnAfterResume = true
					} else {
						return
					}
				case <-pauseTicker.C:
					queueSize := archiver.GetWARCWritingQueueSize()

					logger.Debug("checking queue size for pause", "queue_size", queueSize, "max_queue_size", config.Get().WorkersCount, "paused", paused)

					if !paused && queueSize > maxQueueSize {
						logger.Warn("WARC writing queue exceeded the worker count, pausing the pipeline")
						pause.Pause("WARC writing queue exceeded the worker count")
						paused = true
						lastPauseTime = time.Now()
					} else if paused && time.Since(lastPauseTime) >= pauseTimeout && queueSize < config.Get().WorkersCount {
						logger.Info("WARC writing queue size returned to acceptable, resuming the pipeline")
						pause.Resume()
						paused = false
						if returnAfterResume {
							return
						}
					}
				}
			}
		}()
	}

	// Update the stats every statsUpdateInterval
	wwqWg.Add(1)
	go func() {
		defer wwqWg.Done()

		logger := log.NewFieldedLogger(&log.Fields{
			"component": "controler.warcWritingQueueWatcher.stats",
		})
		defer logger.Debug("closed")

		statsTicker := time.NewTicker(statsUpdateInterval)
		defer statsTicker.Stop()

		for {
			select {
			case <-wwqCtx.Done():
				return
			case <-statsTicker.C:
				queueSize := archiver.GetWARCWritingQueueSize()

				stats.WarcWritingQueueSizeSet(int64(queueSize))

				// Update dedup WARC metrics
				stats.WARCDataTotalBytesSet(archiver.GetWARCTotalBytesArchived())
				stats.WARCDataTotalBytesContentLengthSet(archiver.GetWARCTotalBytesContentLengthArchived())
				stats.WARCCDXDedupeTotalBytesSet(archiver.GetWARCCDXDedupeTotalBytes())
				stats.WARCDoppelgangerDedupeTotalBytesSet(archiver.GetWARCDoppelgangerDedupeTotalBytes())
				stats.WARCLocalDedupeTotalBytesSet(archiver.GetWARCLocalDedupeTotalBytes())
				stats.WARCCDXDedupeTotalSet(archiver.GetWARCCDXDedupeTotal())
				stats.WARCDoppelgangerDedupeTotalSet(archiver.GetWARCDoppelgangerDedupeTotal())
				stats.WARCLocalDedupeTotalSet(archiver.GetWARCLocalDedupeTotal())
			}
		}
	}()
}

// StopWARCWritingQueueWatcher stops the WARC writing queue watcher by canceling the context and waiting for the goroutine to finish
func StopWARCWritingQueueWatcher() {
	wwqCancel()
	wwqWg.Wait()
}
