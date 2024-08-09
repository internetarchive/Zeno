package queue

import (
	"fmt"
	"time"

	"github.com/internetarchive/Zeno/internal/stats"
)

func (q *PersistentGroupedQueue) updateDequeueStats(host string) error {
	if host == "" {
		return fmt.Errorf("host is nil")
	}

	if stats.GetDequeueCount() == 0 {
		stats.SetFirstDequeueTime(time.Now())
	}

	// Update the total elements count
	// Update the total number of elements
	// Update the number of unique hosts
	stats.UpdateElementsPerHost(host, -1)

	stats.UpdateDequeueCount(1)
	stats.SetLastDequeueTime(time.Now())

	return nil
}

func updateEnqueueStats(items ...*Item) error {
	var err error

	for _, item := range items {
		if item == nil {
			err = ErrNilItem
			continue
		}

		if stats.GetEnqueueCount() == 0 {
			stats.SetFirstEnqueueTime(time.Now())
		}

		// Update the total elements count
		// Update the total number of elements
		// Update the number of unique hosts
		stats.UpdateElementsPerHost(item.URL.Host, 1)

		stats.UpdateEnqueueCount(1)
		stats.SetLastEnqueueTime(time.Now())
	}

	return err
}
