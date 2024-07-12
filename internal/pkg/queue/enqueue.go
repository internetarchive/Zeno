package queue

import (
	"fmt"
	"io"
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
	updateEnqueueStats(q, item)

	// Signal that a new item is available
	q.cond.Signal()

	return q.saveMetadata()
}
