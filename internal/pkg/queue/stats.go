package queue

import (
	"sort"
	"time"
)

type QueueStats struct {
	TotalElements              int                `json:"total_elements"`
	UniqueHosts                int                `json:"unique_hosts"`
	ElementsPerHost            map[string]int     `json:"elements_per_host"`
	TotalSize                  uint64             `json:"total_size"`
	UsedSize                   uint64             `json:"used_size"`
	FreeSize                   uint64             `json:"free_size"`
	Utilization                float64            `json:"utilization"`
	EnqueueCount               int                `json:"enqueue_count"`
	DequeueCount               int                `json:"dequeue_count"`
	FirstEnqueueTime           time.Time          `json:"first_enqueue_time"`
	LastEnqueueTime            time.Time          `json:"last_enqueue_time"`
	FirstDequeueTime           time.Time          `json:"first_dequeue_time"`
	LastDequeueTime            time.Time          `json:"last_dequeue_time"`
	AverageTimeBetweenEnqueues time.Duration      `json:"average_time_between_enqueues"`
	AverageTimeBetweenDequeues time.Duration      `json:"average_time_between_dequeues"`
	TopHosts                   []HostStat         `json:"top_hosts"`
	HostDistribution           map[string]float64 `json:"host_distribution"`
	AverageElementsPerHost     float64            `json:"average_elements_per_host"`
}

type HostStat struct {
	Host     string `json:"host"`
	Elements int    `json:"elements"`
}

func (q *PersistentGroupedQueue) GetStats() QueueStats {
	q.statsMutex.RLock()
	defer q.statsMutex.RUnlock()

	// Create a copy of the current stats
	stats := q.stats

	// Calculate top hosts
	var topHosts []HostStat
	for host, count := range stats.ElementsPerHost {
		topHosts = append(topHosts, HostStat{Host: host, Elements: count})
	}

	// Sort topHosts by Elements in descending order
	sort.Slice(topHosts, func(i, j int) bool {
		return topHosts[i].Elements > topHosts[j].Elements
	})

	// Take top 10 or less
	if len(topHosts) > 10 {
		topHosts = topHosts[:10]
	}
	stats.TopHosts = topHosts

	// Calculate host distribution
	stats.HostDistribution = make(map[string]float64)
	if stats.TotalElements > 0 {
		for host, count := range stats.ElementsPerHost {
			stats.HostDistribution[host] = float64(count) / float64(stats.TotalElements)
		}
	}

	// Calculate additional stats
	stats.FreeSize = stats.TotalSize - stats.UsedSize
	if stats.TotalSize > 0 {
		stats.Utilization = float64(stats.UsedSize) / float64(stats.TotalSize)
	} else {
		stats.Utilization = 0
	}
	if stats.UniqueHosts > 0 {
		stats.AverageElementsPerHost = float64(stats.TotalElements) / float64(stats.UniqueHosts)
	} else {
		stats.AverageElementsPerHost = 0
	}

	if stats.DequeueCount > 0 {
		stats.AverageTimeBetweenDequeues = time.Since(stats.FirstDequeueTime) / time.Duration(stats.DequeueCount)
	}
	if stats.EnqueueCount > 0 {
		stats.AverageTimeBetweenEnqueues = time.Since(stats.FirstEnqueueTime) / time.Duration(stats.EnqueueCount)
	}

	return stats
}
