package queue

import (
	"encoding/json"
	"os"
	"sort"
	"time"
)

type QueueStats struct {
	FirstEnqueueTime           time.Time          `json:"first_enqueue_time"`
	LastEnqueueTime            time.Time          `json:"last_enqueue_time"`
	FirstDequeueTime           time.Time          `json:"first_dequeue_time"`
	LastDequeueTime            time.Time          `json:"last_dequeue_time"`
	ElementsPerHost            map[string]int     `json:"elements_per_host"`
	HostDistribution           map[string]float64 `json:"host_distribution"`
	TopHosts                   []HostStat         `json:"top_hosts"`
	TotalElements              int                `json:"total_elements"`
	UniqueHosts                int                `json:"unique_hosts"`
	EnqueueCount               int                `json:"enqueue_count"`
	DequeueCount               int                `json:"dequeue_count"`
	AverageTimeBetweenEnqueues time.Duration      `json:"average_time_between_enqueues"`
	AverageTimeBetweenDequeues time.Duration      `json:"average_time_between_dequeues"`
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

func (q *PersistentGroupedQueue) loadStatsFromFile(path string) error {
	q.statsMutex.Lock()
	defer q.statsMutex.Unlock()

	// Load the stats from the file
	f, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Decode the stats
	err = json.NewDecoder(f).Decode(&q.stats)
	if err != nil {
		return err
	}

	// Delete the file after reading it
	err = os.Remove(path)
	if err != nil {
		return err
	}

	return nil
}

func (q *PersistentGroupedQueue) saveStatsToFile(path string) error {
	q.statsMutex.RLock()
	defer q.statsMutex.RUnlock()

	// Save the stats to the file, creating it if it doesn't exist
	// or truncating it if it does
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Encode the stats
	err = json.NewEncoder(f).Encode(q.GetStats())
	if err != nil {
		return err
	}

	return nil
}

func (q *PersistentGroupedQueue) updateDequeueStats(host string) {
	q.statsMutex.Lock()
	defer q.statsMutex.Unlock()

	q.stats.TotalElements--
	q.stats.ElementsPerHost[host]--
	if q.stats.DequeueCount == 0 {
		q.stats.FirstDequeueTime = time.Now()
	}
	q.stats.DequeueCount++
	q.stats.LastDequeueTime = time.Now()
	if q.stats.ElementsPerHost[host] == 0 {
		delete(q.stats.ElementsPerHost, host)
		q.stats.UniqueHosts--
	}
}

func (q *PersistentGroupedQueue) updateEnqueueStats(item *Item) {
	q.statsMutex.Lock()
	defer q.statsMutex.Unlock()

	q.stats.TotalElements++
	if q.stats.ElementsPerHost[item.URL.Host] == 0 {
		q.stats.UniqueHosts++ // Increment UniqueHosts when we see a new host
	}
	q.stats.ElementsPerHost[item.URL.Host]++
	if q.stats.EnqueueCount == 0 {
		q.stats.FirstEnqueueTime = time.Now()
	}
	q.stats.EnqueueCount++
	q.stats.LastEnqueueTime = time.Now()
}
