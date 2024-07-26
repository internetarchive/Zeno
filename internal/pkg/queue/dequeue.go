package queue

import (
	"fmt"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/queue/index"
)

// Dequeue removes and returns the next item from the queue
// It blocks until an item is available
func (q *PersistentGroupedQueue) Dequeue() (*Item, error) {
	if !q.CanDequeue() {
		return nil, ErrQueueClosed
	}

	var (
		position = uint64(0)
		size     = uint64(0)
		errPop   error
	)

	host, err := q.getNextHost()
	if err != nil {
		return nil, fmt.Errorf("failed to get next host: %w", err)
	}

	_, position, size, errPop = q.index.Pop(host)
	if errPop != nil && errPop != index.ErrHostEmpty {
		if errPop == index.ErrHostNotFound {
			return q.Dequeue() // Try again with another host, this one might be empty due to the non-blocking nature of getNextHost
		}
		return nil, fmt.Errorf("failed to pop item from host %s: %w", host, errPop)
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()

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

// getNextHost returns the next host to dequeue from
// It blocks until a host is available, allowing other goroutines to enqueue without keeping the queue mutex locked
func (q *PersistentGroupedQueue) getNextHost() (string, error) {
	for q.Empty.Get() && q.CanDequeue() {
		time.Sleep(1 * time.Second)
	}

	// If the queue is closed or finishing, we end the wait loop
	if !q.CanDequeue() {
		return "", ErrQueueClosed
	}

	// Get a copy of the hosts
	hosts := q.index.GetHosts()

	// If there are no hosts, we wait for one to be added
	if len(hosts) == 0 {
		q.Empty.Set(true)
		return q.getNextHost()
	}

	// Magic operating : get the next host to dequeue from
	i := q.currentHost.Load() % uint64(len(hosts))
	q.currentHost.Add(1)
	return hosts[i], nil
}
