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
		position uint64
		size     uint64
		err      error
	)

	q.mutex.Lock()
	defer q.mutex.Unlock()

	if len(q.hostIndex.GetHosts()) == 0 {
		// Maybe should be a more specific error?
		return nil, ErrQueueEmpty
	}

	hosts := q.hostIndex.GetHosts()
	for _, host := range hosts {
		if len(hosts) == 0 {
			return nil, ErrQueueEmpty
		}
		position, size, err = q.hostIndex.Pop(host)
		if err != nil {
			if err == index.ErrHostEmpty {
				continue
			} else if err == index.ErrHostNotFound {
				panic(fmt.Sprintf("host %s not found in index, this indicates a failure in index package logic", host))
			}
			return nil, fmt.Errorf("failed to pop item from host %s: %w", host, err)
		}
		break
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

	updateDequeueStats(q, item.URL.Host)

	return item, nil
}
