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

	host := q.hostOrder[q.currentHost]
	positions := q.hostIndex[host]

	if len(positions) == 0 {
		// Remove this host from the order and index
		q.hostOrder = append(q.hostOrder[:q.currentHost], q.hostOrder[q.currentHost+1:]...)
		delete(q.hostIndex, host)
		if len(q.hostOrder) > 0 {
			q.currentHost = q.currentHost % len(q.hostOrder)
		} else {
			q.currentHost = 0
		}
		return q.dequeue()
	}

	position := positions[0]
	q.hostIndex[host] = positions[1:]

	// Seek to position and decode item
	_, err := q.file.Seek(int64(position), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to item position: %w", err)
	}
	var item Item
	err = q.decoder.Decode(&item)
	if err != nil {
		return nil, fmt.Errorf("failed to decode item: %w", err)
	}

	// Move to next host
	if len(q.hostOrder) > 0 {
		q.currentHost = (q.currentHost + 1) % len(q.hostOrder)
	} else {
		q.currentHost = 0
	}

	// Update stats
	q.statsMutex.Lock()
	q.stats.TotalElements--
	currentPos, _ := q.file.Seek(0, io.SeekCurrent)
	q.stats.UsedSize -= uint64(currentPos - int64(position))
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
