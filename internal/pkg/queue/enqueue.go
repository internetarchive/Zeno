package queue

import (
	"fmt"
	"io"
	"log"
	"time"
)

func (q *PersistentGroupedQueue) Enqueue(item *Item) error {
	if q.closed {
		return ErrQueueClosed
	}
	q.enqueueChan <- item
	return nil
}

func (q *PersistentGroupedQueue) enqueue(item *Item) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	// Find free position
	startPos, err := q.file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("failed to seek to end of file: %w", err)
	}

	// Encode and write item
	err = q.encoder.Encode(item)
	if err != nil {
		return fmt.Errorf("failed to encode item: %w", err)
	}

	endPos, err := q.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get current position: %w", err)
	}
	itemSize := endPos - startPos

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
	q.stats.UsedSize += uint64(itemSize)
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

	return q.saveMetadata()
}

func (q *PersistentGroupedQueue) enqueueWorker() {
	defer q.wg.Done()
	for {
		select {
		case item, ok := <-q.enqueueChan:
			if !ok {
				return
			}
			if q.closed {
				log.Printf("Cannot enqueue item: queue is closed")
				continue
			}
			err := q.enqueue(item)
			if err != nil {
				log.Printf("Error enqueueing item: %v", err)
			}
		case <-q.done:
			return
		}
	}
}
