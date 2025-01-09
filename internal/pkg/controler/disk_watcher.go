package controler

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
)

var (
	diskWatcherCtx, diskWatcherCancel = context.WithCancel(context.Background())
)

// Implements f(x)={ if total <= 256GB then threshold = 50GB * (total / 256GB) else threshold = 50GB }
func checkDiskUsage(total, free uint64) error {
	const (
		GB = 1024 * 1024 * 1024
	)
	var threshold float64

	if total <= 256*GB {
		threshold = float64(50*GB) * (float64(total) / float64(256*GB))
	} else {
		threshold = 50 * GB
	}

	// Compare free space with threshold
	if free < uint64(threshold) {
		return fmt.Errorf("low disk space: free=%.2f GB, threshold=%.2f GB", float64(free)/1e9, float64(threshold)/1e9)
	}

	return nil
}

func watchDiskSpace(path string, interval time.Duration) {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "controler.diskWatcher",
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
			var stat syscall.Statfs_t
			if err := syscall.Statfs(path, &stat); err != nil {
				logger.Error("Error retrieving disk stats: %v\n", err)
				continue
			}

			total := stat.Blocks * uint64(stat.Bsize)
			free := stat.Bavail * uint64(stat.Bsize)

			err := checkDiskUsage(total, free)

			if err != nil && !paused {
				logger.Warn("Low disk space, pausing the pipeline", "err", err.Error())
				pause.Pause("Not enough disk space!!!")
				paused = true
			} else if err == nil && paused {
				logger.Info("Disk space is sufficient, resuming the pipeline")
				pause.Resume()
				paused = false
				if returnASAP {
					return
				}
			}
		}
	}
}
