package controler

import (
	"context"
	"fmt"
	"log"
	"syscall"
	"time"
)

var (
	diskWatcherCtx, diskWatcherCancel = context.WithCancel(context.Background())
)

// StartDiskWatcher starts a goroutine that checks the disk space
// Implements f(x)={ if total <= 256GB then threshold = 20GB * (total / 256GB) else threshold = 20GB }
func checkDiskUsage(total, free uint64) error {
	const (
		GB = 1024 * 1024 * 1024
	)
	var threshold float64

	if total <= 256*GB {
		threshold = float64(20*GB) * (float64(total) / float64(256*GB))
	} else {
		threshold = 20 * GB
	}

	// Compare free space with threshold
	if free < uint64(threshold) {
		return fmt.Errorf("low disk space: free=%.2f GB, threshold=%.2f GB", float64(free)/1e9, float64(threshold)/1e9)
	}

	return nil
}

func watchDiskSpace(path string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-diskWatcherCtx.Done():
			return
		case <-ticker.C:
			var stat syscall.Statfs_t
			if err := syscall.Statfs(path, &stat); err != nil {
				log.Printf("Error retrieving disk stats: %v\n", err)
				continue
			}

			total := stat.Blocks * uint64(stat.Bsize)
			free := stat.Bavail * uint64(stat.Bsize)

			if err := checkDiskUsage(total, free); err != nil {
				log.Printf("Error checking disk usage: %v\n", err)
			}
		}
	}
}
