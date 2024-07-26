package queue

import (
	"fmt"

	"github.com/internetarchive/Zeno/internal/pkg/queue/index"
)

// Dequeue removes and returns the next item from the queue
// It blocks until an item is available
func (q *PersistentGroupedQueue) Dequeue() (*Item, error) {
	if q.closed {
		return nil, ErrQueueClosed
	}

	var (
		position = uint64(0)
		size     = uint64(0)
		err      error
	)

	if q.Empty.Get() {
		return nil, ErrQueueEmpty
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()

	hosts := q.index.GetHosts()

	if len(hosts) == 0 {
		q.Empty.Set(true)
		return nil, ErrQueueEmpty
	}

	for _, host := range hosts {
		_, position, size, err = q.index.Pop(host)
		if err != nil {
			if err == index.ErrHostEmpty {
				continue
			} else if err == index.ErrHostNotFound {
				return nil, fmt.Errorf("host %s not found in index, this indicates a failure in index package logic", host)
			}
			return nil, fmt.Errorf("failed to pop item from host %s: %w", host, err)
		}
		break
	}

	if position == 0 && size == 0 {
		q.Empty.Set(true)
		return nil, ErrQueueEmpty
	}

	// Read and unmarshal the item
	itemBytes, err := q.ReadItemAt(position, size)
	if err != nil {
		return nil, fmt.Errorf("failed to read item at position %d: %w", position, err)
	}

	item, err := decodeProtoItem(itemBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal item: %w", err)
	}

	q.updateDequeueStats(item.URL.Host)

	return item, nil
}
