package queue

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"
)

type Sample struct {
	timestamp time.Time
	duration  time.Duration
}

type Window struct {
	duration time.Duration
	count    int
	sum      time.Duration
}

type OpAverageDuration struct {
	Minute   time.Duration `json:"minute"`
	Minute10 time.Duration `json:"minute_10"`
	Minute30 time.Duration `json:"minute_30"`
	Hour     time.Duration `json:"hour"`
}

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

	// Sample durations used to calculate average operation durations
	EnqueueSamples            []Sample `json:"enqueue_samples"`
	DequeueSamples            []Sample `json:"dequeue_samples"`
	UpdateEnqueueStatsSamples []Sample `json:"update_enqueue_stats_samples"`
	UpdateDequeueStatsSamples []Sample `json:"update_dequeue_stats_samples"`

	AverageEnqueueDuration            OpAverageDuration `json:"average_enqueue_duration"`
	AverageDequeueDuration            OpAverageDuration `json:"average_dequeue_duration"`
	AverageUpdateEnqueueStatsDuration OpAverageDuration `json:"average_update_enqueue_stats_duration"`
	AverageUpdateDequeueStatsDuration OpAverageDuration `json:"average_update_dequeue_stats_duration"`
}

type HostStat struct {
	Host     string `json:"host"`
	Elements int    `json:"elements"`
}

type SampleType int

const (
	EnqueueSample SampleType = iota
	DequeueSample
	UpdateEnqueueStatsSample
	UpdateDequeueStatsSample
)

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

	// Calculate additional stats
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

	// Calculate average operation durations
	q.stats.AverageEnqueueDuration = q.calculateAverageDuration(q.stats.EnqueueSamples)
	q.stats.AverageDequeueDuration = q.calculateAverageDuration(q.stats.DequeueSamples)
	q.stats.AverageUpdateEnqueueStatsDuration = q.calculateAverageDuration(q.stats.UpdateEnqueueStatsSamples)
	q.stats.AverageUpdateDequeueStatsDuration = q.calculateAverageDuration(q.stats.UpdateDequeueStatsSamples)
}

func (q *PersistentGroupedQueue) calculateAverageDuration(samples []Sample) OpAverageDuration {
	opAvgDuration := OpAverageDuration{}

	if len(samples) == 0 {
		return opAvgDuration
	}

	var totalMinute, totalMinute10, totalMinute30, totalHour time.Duration
	var countMinute, countMinute10, countMinute30, countHour int

	now := time.Now()

	for _, sample := range samples {
		if sample.timestamp.After(now.Add(-1 * time.Minute)) {
			totalMinute += sample.duration
			countMinute++
		}
		if sample.timestamp.After(now.Add(-10 * time.Minute)) {
			totalMinute10 += sample.duration
			countMinute10++
		}
		if sample.timestamp.After(now.Add(-30 * time.Minute)) {
			totalMinute30 += sample.duration
			countMinute30++
		}
		if sample.timestamp.After(now.Add(-1 * time.Hour)) {
			totalHour += sample.duration
			countHour++
		}
	}

	if countMinute > 0 {
		opAvgDuration.Minute = totalMinute / time.Duration(countMinute)
	}
	if countMinute10 > 0 {
		opAvgDuration.Minute10 = totalMinute10 / time.Duration(countMinute10)
	}
	if countMinute30 > 0 {
		opAvgDuration.Minute30 = totalMinute30 / time.Duration(countMinute30)
	}
	if countHour > 0 {
		opAvgDuration.Hour = totalHour / time.Duration(countHour)
	}

	return opAvgDuration
}

func (q *PersistentGroupedQueue) addSample(duration time.Duration, sampleType SampleType) {
	newSample := Sample{
		timestamp: time.Now(),
		duration:  duration,
	}

	switch sampleType {
	case EnqueueSample:
		q.stats.EnqueueSamples = append(q.stats.EnqueueSamples, newSample)
	case DequeueSample:
		q.stats.DequeueSamples = append(q.stats.DequeueSamples, newSample)
	case UpdateEnqueueStatsSample:
		q.stats.UpdateEnqueueStatsSamples = append(q.stats.UpdateEnqueueStatsSamples, newSample)
	case UpdateDequeueStatsSample:
		q.stats.UpdateDequeueStatsSamples = append(q.stats.UpdateDequeueStatsSamples, newSample)
	}

	// Remove old samples to prevent unbounded growth
	q.cleanupSamples(sampleType)
}

func (q *PersistentGroupedQueue) cleanupSamples(sampleType SampleType) {
	threshold := time.Now().Add(-time.Hour)

	cleanup := func(samples []Sample) []Sample {
		newSamples := make([]Sample, 0, len(samples))
		for _, sample := range samples {
			if sample.timestamp.After(threshold) {
				newSamples = append(newSamples, sample)
			}
		}
		return newSamples
	}

	switch sampleType {
	case EnqueueSample:
		q.stats.EnqueueSamples = cleanup(q.stats.EnqueueSamples)
	case DequeueSample:
		q.stats.DequeueSamples = cleanup(q.stats.DequeueSamples)
	case UpdateEnqueueStatsSample:
		q.stats.UpdateEnqueueStatsSamples = cleanup(q.stats.UpdateEnqueueStatsSamples)
	case UpdateDequeueStatsSample:
		q.stats.UpdateDequeueStatsSamples = cleanup(q.stats.UpdateDequeueStatsSamples)
	}
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
	startTime := time.Now()

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

	q.addSample(time.Since(startTime), UpdateDequeueStatsSample)
}

func (q *PersistentGroupedQueue) updateEnqueueStats(item *Item) {
	startTime := time.Now()

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

	q.addSample(time.Since(startTime), UpdateEnqueueStatsSample)
}
