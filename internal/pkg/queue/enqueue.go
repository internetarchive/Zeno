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
	err = q.index.Add(item.URL.Host, item.ID, uint64(startPos), uint64(len(itemBytes)))
	if err != nil {
		return fmt.Errorf("failed to update index: %w", err)
	}

	// Update stats
	q.updateEnqueueStats(item)

	// Update empty status
	q.Empty.Set(false)

	return nil
}
