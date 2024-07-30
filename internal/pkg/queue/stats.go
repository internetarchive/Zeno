package queue

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"
)

type QueueStats struct {
	sync.Mutex `json:"-"`

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
	HandoverSuccessGetCount    uint64             `json:"handover_success_get_count"`
}

type HostStat struct {
	Host     string `json:"host"`
	Elements int    `json:"elements"`
}

func (q *PersistentGroupedQueue) GetStats() *QueueStats {
	q.genStats()
	return q.stats
}

// genStats is not thread-safe and should be called with the statsMutex locked
func (q *PersistentGroupedQueue) genStats() {
	q.stats.Lock()
	defer q.stats.Unlock()

	// Calculate top hosts
	var topHosts []HostStat
	for host, count := range q.stats.ElementsPerHost {
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
	q.stats.TopHosts = topHosts

	// Calculate host distribution
	q.stats.HostDistribution = make(map[string]float64)
	if q.stats.TotalElements > 0 {
		for host, count := range q.stats.ElementsPerHost {
			q.stats.HostDistribution[host] = float64(count) / float64(q.stats.TotalElements)
		}
	}

	// Calculate additional q.Stats
	if q.stats.UniqueHosts > 0 {
		q.stats.AverageElementsPerHost = float64(q.stats.TotalElements) / float64(q.stats.UniqueHosts)
	} else {
		q.stats.AverageElementsPerHost = 0
	}

	if q.stats.DequeueCount > 0 {
		q.stats.AverageTimeBetweenDequeues = time.Since(q.stats.FirstDequeueTime) / time.Duration(q.stats.DequeueCount)
	}
	if q.stats.EnqueueCount > 0 {
		q.stats.AverageTimeBetweenEnqueues = time.Since(q.stats.FirstEnqueueTime) / time.Duration(q.stats.EnqueueCount)
	}

	// Calculate handover success get count
	q.stats.HandoverSuccessGetCount = q.Handover.count.Load()
}

func (q *PersistentGroupedQueue) loadStatsFromFile(path string) error {
	q.stats.Lock()
	defer q.stats.Unlock()

	// Load the stats from the file
	f, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Decode the stats
	err = json.NewDecoder(f).Decode(q.stats)
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
	// Save the stats to the file, creating it if it doesn't exist or truncating it if it does
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
	q.stats.Lock()
	defer q.stats.Unlock()

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
	q.stats.Lock()
	defer q.stats.Unlock()

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
