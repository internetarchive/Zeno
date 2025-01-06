package controler

import (
	"context"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/archiver"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
)

var (
	wwqContext, wwqCancel = context.WithCancel(context.Background())
)

func watchWARCWritingQueue(interval time.Duration) {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "controler.warcWritingQueueWatcher",
	})

	paused := false
	returnASAP := false
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-diskWatcherCtx.Done():
			defer logger.Debug("closed")
			if paused {
				logger.Info("returning after resume")
				returnASAP = true
			}
			return
		case <-ticker.C:
			queueSize := archiver.GetWARCWritingQueueSize()

			if queueSize >= (config.Get().WorkersCount*4) && !paused {
				logger.Warn("WARC writing queue exceeded 4x the worker count, pausing the pipeline")
				pause.Pause()
				paused = true
			} else if queueSize < (config.Get().WorkersCount*4) && paused {
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
