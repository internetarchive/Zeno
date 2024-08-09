package stats

import (
	"encoding/json"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type queueStats struct {
	// Time
	firstEnqueueTime *atomic.Value // stores time.Time
	lastEnqueueTime  *atomic.Value // stores time.Time
	firstDequeueTime *atomic.Value // stores time.Time
	lastDequeueTime  *atomic.Value // stores time.Time

	// Elements
	elementsPerHost sync.Map // map[string]*atomic.Int64
	totalElements   *atomic.Int64
	uniqueHosts     *atomic.Int64

	// Counts
	enqueueCount            *atomic.Int64
	dequeueCount            *atomic.Int64
	handoverSuccessGetCount *atomic.Int64

	// Status
	handoverOpen *atomic.Bool
	queueEmpty   *atomic.Bool
	canEnqueue   *atomic.Bool
	canDequeue   *atomic.Bool
}

// JSONCompatibleQueueStats is a struct that holds queue stats to be JSON encoded
type JSONCompatibleQueueStats struct {
	FirstEnqueueTime           int64 `json:"first_enqueue_time_unixns"`
	LastEnqueueTime            int64 `json:"last_enqueue_time_unixns"`
	FirstDequeueTime           int64 `json:"first_dequeue_time_unixns"`
	LastDequeueTime            int64 `json:"last_dequeue_time_unixns"`
	TotalElements              int64 `json:"total_elements"`
	UniqueHosts                int64 `json:"unique_hosts"`
	EnqueueCount               int64 `json:"enqueue_count"`
	DequeueCount               int64 `json:"dequeue_count"`
	AverageTimeBetweenEnqueues int64 `json:"average_time_between_enqueues_ns"`
	AverageTimeBetweenDequeues int64 `json:"average_time_between_dequeues_ns"`
	AverageElementsPerHost     int64 `json:"average_elements_per_host"`
	HandoverSuccessGetCount    int64 `json:"handover_success_get_count"`
}

func newQueueStats() *queueStats {
	return &queueStats{
		firstEnqueueTime:        new(atomic.Value),
		lastEnqueueTime:         new(atomic.Value),
		firstDequeueTime:        new(atomic.Value),
		lastDequeueTime:         new(atomic.Value),
		elementsPerHost:         sync.Map{},
		totalElements:           new(atomic.Int64),
		uniqueHosts:             new(atomic.Int64),
		enqueueCount:            new(atomic.Int64),
		dequeueCount:            new(atomic.Int64),
		handoverSuccessGetCount: new(atomic.Int64),
		handoverOpen:            new(atomic.Bool),
		queueEmpty:              new(atomic.Bool),
		canEnqueue:              new(atomic.Bool),
		canDequeue:              new(atomic.Bool),
	}
}

// GetJSONQueueStats returns a JSONCompatibleQueueStats struct
func GetJSONQueueStats() *JSONCompatibleQueueStats {
	computedStats := &JSONCompatibleQueueStats{
		FirstEnqueueTime:        GetFirstEnqueueTime().UnixNano(),
		LastEnqueueTime:         GetLastEnqueueTime().UnixNano(),
		FirstDequeueTime:        GetFirstDequeueTime().UnixNano(),
		LastDequeueTime:         GetLastDequeueTime().UnixNano(),
		TotalElements:           GetQueueTotalElementsCount(),
		UniqueHosts:             GetQueueUniqueHostsCount(),
		EnqueueCount:            GetEnqueueCount(),
		DequeueCount:            GetDequeueCount(),
		HandoverSuccessGetCount: GetHandoverSuccessGetCount(),
	}
	// Compute averages
	if computedStats.UniqueHosts > 0 {
		computedStats.AverageElementsPerHost = computedStats.TotalElements / computedStats.UniqueHosts
	} else {
		computedStats.AverageElementsPerHost = 0
	}

	if computedStats.DequeueCount > 0 {
		computedStats.AverageTimeBetweenDequeues = computedStats.FirstDequeueTime / time.Duration(computedStats.DequeueCount).Nanoseconds()
	} else {
		computedStats.AverageTimeBetweenDequeues = 0
	}

	if computedStats.EnqueueCount > 0 {
		computedStats.AverageTimeBetweenEnqueues = computedStats.FirstEnqueueTime / time.Duration(computedStats.EnqueueCount).Nanoseconds()
	} else {
		computedStats.AverageTimeBetweenEnqueues = 0
	}

	return computedStats
}

//////////////////////////////////
// Setters and Getters for Time //
//////////////////////////////////

// SetFirstEnqueueTime sets the first enqueue time
func SetFirstEnqueueTime(t time.Time) {
	packageRunner.data.queue.firstEnqueueTime.Store(t)
}

// GetFirstEnqueueTime returns the first enqueue time
func GetFirstEnqueueTime() time.Time {
	v, ok := packageRunner.data.queue.firstEnqueueTime.Load().(time.Time)
	if !ok {
		return time.Time{}
	}
	return v
}

// SetLastEnqueueTime sets the last enqueue time
func SetLastEnqueueTime(t time.Time) {
	packageRunner.data.queue.lastEnqueueTime.Store(t)
}

// GetLastEnqueueTime returns the last enqueue time
func GetLastEnqueueTime() time.Time {
	v, ok := packageRunner.data.queue.lastEnqueueTime.Load().(time.Time)
	if !ok {
		return time.Time{}
	}
	return v
}

// SetFirstDequeueTime sets the first dequeue time
func SetFirstDequeueTime(t time.Time) {
	packageRunner.data.queue.firstDequeueTime.Store(t)
}

// GetFirstDequeueTime returns the first dequeue time
func GetFirstDequeueTime() time.Time {
	v, ok := packageRunner.data.queue.firstDequeueTime.Load().(time.Time)
	if !ok {
		return time.Time{}
	}
	return v
}

// SetLastDequeueTime sets the last dequeue time
func SetLastDequeueTime(t time.Time) {
	packageRunner.data.queue.lastDequeueTime.Store(t)
}

// GetLastDequeueTime returns the last dequeue time
func GetLastDequeueTime() time.Time {
	v, ok := packageRunner.data.queue.lastDequeueTime.Load().(time.Time)
	if !ok {
		return time.Time{}
	}
	return v
}

///////////////////////////////
// Elements And Host Methods //
///////////////////////////////

// UpdateElementsPerHost updates the number of elements per host
// Features:
//   - If the host does not exist, it will be created
//   - If the number of elements is 0, the host will be deleted
//   - The total number of elements will be updated
//   - The number of unique hosts will be updated
//   - Delta can be positive or negative
func UpdateElementsPerHost(host string, delta int64) {
	v, ok := packageRunner.data.queue.elementsPerHost.Load(host)
	if !ok {
		newCounter := &atomic.Int64{}
		actual, loaded := packageRunner.data.queue.elementsPerHost.LoadOrStore(host, newCounter)
		if !loaded {
			packageRunner.data.queue.uniqueHosts.Add(1)
		}
		v = actual
	}
	counter := v.(*atomic.Int64)
	newVal := counter.Add(delta)

	if newVal == 0 {
		if packageRunner.data.queue.elementsPerHost.CompareAndDelete(host, v) {
			packageRunner.data.queue.uniqueHosts.Add(-1)
		}
	}

	packageRunner.data.queue.totalElements.Add(delta)
}

// GetElementsPerHost returns the number of elements per host as a map[string]int64
func GetElementsPerHost() map[string]int64 {
	result := make(map[string]int64)
	packageRunner.data.queue.elementsPerHost.Range(func(key, value interface{}) bool {
		result[key.(string)] = value.(*atomic.Int64).Load()
		return true
	})
	return result
}

// GetQueueTotalElementsCount returns the total number of elements
func GetQueueTotalElementsCount() int64 {
	return packageRunner.data.queue.totalElements.Load()
}

// GetQueueUniqueHostsCount returns the number of unique hosts
func GetQueueUniqueHostsCount() int64 {
	return packageRunner.data.queue.uniqueHosts.Load()
}

////////////////////////////////////
// Setters and Getters for Counts //
////////////////////////////////////

// UpdateEnqueueCount adds delta to the enqueue count. Delta can be positive or negative.
func UpdateEnqueueCount(delta int64) {
	packageRunner.data.queue.enqueueCount.Add(delta)
}

// GetEnqueueCount returns the enqueue count
func GetEnqueueCount() int64 {
	return packageRunner.data.queue.enqueueCount.Load()
}

// UpdateDequeueCount adds delta to the dequeue count. Delta can be positive or negative.
func UpdateDequeueCount(delta int64) {
	packageRunner.data.queue.dequeueCount.Add(delta)
}

// GetDequeueCount returns the dequeue count
func GetDequeueCount() int64 {
	return packageRunner.data.queue.dequeueCount.Load()
}

// UpdateHandoverSuccessGetCount adds delta to the handover success get count. Delta can be positive or negative.
func UpdateHandoverSuccessGetCount(delta int64) {
	packageRunner.data.queue.handoverSuccessGetCount.Add(delta)
}

// GetHandoverSuccessGetCount returns the handover success get count
func GetHandoverSuccessGetCount() int64 {
	return packageRunner.data.queue.handoverSuccessGetCount.Load()
}

/////////////////////////////////////
//   Queue save and load methods   //
/////////////////////////////////////

// LoadQueueStatsFromFile loads the queue stats from a file
func LoadQueueStatsFromFile(path string) error {
	if packageRunner == nil || packageRunner.data == nil || packageRunner.data.queue == nil {
		return ErrStatsNotInitialized
	}

	// Load the stats from the file
	f, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Decode the stats
	err = json.NewDecoder(f).Decode(packageRunner.data.queue)
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

// SaveQueueStatsToFile saves the queue stats to a file
func SaveQueueStatsToFile(path string) error {
	if packageRunner == nil || packageRunner.data == nil || packageRunner.data.queue == nil {
		return ErrStatsNotInitialized
	}

	// Save the stats to the file, creating it if it doesn't exist or truncating it if it does
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Encode the stats
	err = json.NewEncoder(f).Encode(GetJSONQueueStats())
	if err != nil {
		return err
	}

	return nil
}

//////////////
//  Status  //
//////////////

// SetHandoverOpen sets the handover open status
func SetHandoverOpen(open bool) {
	packageRunner.data.queue.handoverOpen.Store(open)
}

// GetHandoverOpen returns the handover open status
func GetHandoverOpen() bool {
	return packageRunner.data.queue.handoverOpen.Load()
}

// SetQueueEmpty sets the queue empty status
func SetQueueEmpty(empty bool) {
	packageRunner.data.queue.queueEmpty.Store(empty)
}

// GetQueueEmpty returns the queue empty status
func GetQueueEmpty() bool {
	return packageRunner.data.queue.queueEmpty.Load()
}

// SetCanEnqueue sets the can enqueue status
func SetCanEnqueue(canEnqueue bool) {
	packageRunner.data.queue.canEnqueue.Store(canEnqueue)
}

// GetCanEnqueue returns the can enqueue status
func GetCanEnqueue() bool {
	return packageRunner.data.queue.canEnqueue.Load()
}

// SetCanDequeue sets the can dequeue status
func SetCanDequeue(canDequeue bool) {
	packageRunner.data.queue.canDequeue.Store(canDequeue)
}

// GetCanDequeue returns the can dequeue status
func GetCanDequeue() bool {
	return packageRunner.data.queue.canDequeue.Load()
}
