package queue

import (
	"fmt"
	"io"
	"time"
)

func (q *PersistentGroupedQueue) Enqueue(item *Item) error {
	if q.closed {
		return ErrQueueClosed
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()

	// Find free position
	startPos, err := q.queueFile.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("failed to seek to end of file: %w", err)
	}

	// Encode and write item
	err = q.queueEncoder.Encode(item)
	if err != nil {
		return fmt.Errorf("failed to encode item: %w", err)
	}

	// Update host index and order
	q.hostMutex.Lock()
	if _, exists := q.hostIndex[item.Host]; !exists {
		q.hostOrder = append(q.hostOrder, item.Host)
	}
	q.hostIndex[item.Host] = append(q.hostIndex[item.Host], uint64(startPos))
	q.hostMutex.Unlock()

	// Update stats
	q.statsMutex.Lock()
	q.stats.TotalElements++
	q.stats.ElementsPerHost[item.Host]++
	if q.stats.EnqueueCount == 0 {
		q.stats.FirstEnqueueTime = time.Now()
	}
	q.stats.EnqueueCount++
	q.stats.LastEnqueueTime = time.Now()
	if q.stats.ElementsPerHost[item.Host] == 1 {
		q.stats.UniqueHosts++
	}
	q.statsMutex.Unlock()

	// Signal that a new item is available
	q.cond.Signal()

	return q.saveMetadata()
}
