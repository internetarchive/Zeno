package queue

import (
	"fmt"
	"io"
	"log"
	"time"
)

func (q *PersistentGroupedQueue) Dequeue() (*Item, error) {
	if q.closed {
		return nil, ErrQueueClosed
	}

	resultChan := make(chan *Item, 1)

	go func() {
		q.dequeueChan <- resultChan
	}()

	select {
	case item := <-resultChan:
		if item == nil {
			return nil, ErrQueueEmpty
		}
		return item, nil
	case <-time.After(5 * time.Second):
		return nil, ErrQueueTimeout
	case <-q.done:
		return nil, ErrQueueClosed
	}
}

func (q *PersistentGroupedQueue) dequeue() (*Item, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	q.hostMutex.Lock()
	defer q.hostMutex.Unlock()

	if len(q.hostOrder) == 0 {
		return nil, ErrQueueEmpty
	}

	// Loop through hosts until we find one with items or we've checked all hosts
	hostsChecked := 0
	for hostsChecked < len(q.hostOrder) {
		host := q.hostOrder[q.currentHost]
		positions := q.hostIndex[host]

		if len(positions) == 0 {
			// Remove this host from the order and index
			q.hostOrder = append(q.hostOrder[:q.currentHost], q.hostOrder[q.currentHost+1:]...)
			delete(q.hostIndex, host)
			if len(q.hostOrder) == 0 {
				q.currentHost = 0
				return nil, ErrQueueEmpty
			}
			q.currentHost = q.currentHost % len(q.hostOrder)
			hostsChecked++
			continue
		}

		// We found a host with items, dequeue from here
		position := positions[0]
		q.hostIndex[host] = positions[1:]

		// Seek to position and decode item
		_, err := q.queueFile.Seek(int64(position), io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("failed to seek to item position: %w", err)
		}
		var item Item
		err = q.queueDecoder.Decode(&item)
		if err != nil {
			return nil, fmt.Errorf("failed to decode item: %w", err)
		}

		// Move to next host
		q.currentHost = (q.currentHost + 1) % len(q.hostOrder)

		// Update stats
		q.statsMutex.Lock()
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
		q.statsMutex.Unlock()

		return &item, q.saveMetadata()
	}

	// If we've checked all hosts and found no items, the queue is empty
	return nil, ErrQueueEmpty
}

func (q *PersistentGroupedQueue) dequeueWorker() {
	defer q.wg.Done()
	for {
		select {
		case resultChan, ok := <-q.dequeueChan:
			if !ok {
				return
			}
			if q.closed {
				resultChan <- nil
				continue
			}
			item, err := q.dequeue()
			if err != nil {
				if err == ErrQueueEmpty {
					resultChan <- nil
				} else {
					log.Printf("Error dequeueing item: %v", err)
					resultChan <- nil
				}
			} else {
				resultChan <- item
			}
		case <-q.done:
			return
		}
	}

}
