package watchers

import (
	"context"
	"fmt"
	"sync"
	"syscall"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
)

var (
	diskWatcherCtx, diskWatcherCancel = context.WithCancel(context.Background())
	diskWatcherWg                     sync.WaitGroup
)

// Implements f(x)={ if total <= 256GB then threshold = 50GB * (total / 256GB) else threshold = 50GB }
func checkThreshold(total, free uint64, minSpaceRequired float64) error {
	const (
		GB = 1024 * 1024 * 1024
	)
	var threshold float64

	if minSpaceRequired > 0 {
		threshold = float64(minSpaceRequired) * float64(GB)
	} else {
		if total <= 256*GB {
			threshold = float64(50*GB) * (float64(total) / float64(256*GB))
		} else {
			threshold = 50 * GB
		}
	}

	// Compare free space with threshold
	if free < uint64(threshold) {
		return fmt.Errorf("low disk space: free=%.2f GB, threshold=%.2f GB", float64(free)/1e9, float64(threshold)/1e9)
	}

	return nil
}

func CheckDiskUsage(path string) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		panic(fmt.Sprintf("Error retrieving disk stats: %v\n", err))
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)

	return checkThreshold(total, free, config.Get().MinSpaceRequired)
}

// WatchDiskSpace watches the disk space and pauses the pipeline if it's low
func WatchDiskSpace(path string, interval time.Duration) {
	diskWatcherWg.Add(1)
	defer diskWatcherWg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "controler.diskWatcher",
	})

	paused := false
	returnASAP := false
	backoffMultiplier := 0
	maxInterval := 10 * interval

	for {
		select {
		case <-diskWatcherCtx.Done():
			defer logger.Debug("closed")
			if paused {
				logger.Info("returning after resume")
				returnASAP = true
			}
			return
		default:
			err := CheckDiskUsage(path)

			if err != nil && !paused {
				logger.Warn("Low disk space, pausing the pipeline", "err", err.Error())
				pause.Pause("Not enough disk space!!!")
				paused = true
				backoffMultiplier++
			} else if err == nil && paused {
				logger.Info("Disk space is sufficient, resuming the pipeline")
				pause.Resume()
				paused = false
				backoffMultiplier = 0
				if returnASAP {
					return
				}
			} else if err != nil {
				backoffMultiplier++
			} else {
				backoffMultiplier = 0
			}

			sleep := interval * (1 << backoffMultiplier) // exponential backoff
			if sleep > maxInterval {
				sleep = maxInterval
			}
			time.Sleep(sleep)
		}
	}
}

// StopDiskWatcher stops the disk watcher by canceling the context and waiting for the goroutine to finish.
func StopDiskWatcher() {
	diskWatcherCancel()
	diskWatcherWg.Wait()
}
