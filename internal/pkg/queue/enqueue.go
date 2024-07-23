package queue

import (
	"errors"
	"fmt"
	"io"
)

func (q *PersistentGroupedQueue) Enqueue(item *Item) error {
	if item == nil {
		return errors.New("cannot enqueue nil item")
	}

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
	itemBytes, err := encodeItem(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	_, err = q.queueFile.Write(itemBytes)
	if err != nil {
		return fmt.Errorf("failed to write item: %w", err)
	}

	// Update host index and order
	q.hostMutex.Lock()
	if _, exists := q.hostIndex[item.URL.Host]; !exists {
		q.hostOrder = append(q.hostOrder, item.URL.Host)
	}

	q.hostIndex[item.URL.Host] = append(q.hostIndex[item.URL.Host], uint64(startPos))
	q.hostIndex[item.URL.Host] = append(q.hostIndex[item.URL.Host], uint64(len(itemBytes)))
	q.hostMutex.Unlock()

	// Update stats
	q.updateEnqueueStats(item)

	return nil
}
