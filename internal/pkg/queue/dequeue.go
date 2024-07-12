package queue

import (
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
)

// Dequeue removes and returns the next item from the queue
// It blocks until an item is available
func (q *PersistentGroupedQueue) Dequeue() (*Item, error) {
	if q.closed {
		return nil, ErrQueueClosed
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()

	for len(q.hostOrder) == 0 {
		q.LoggingChan <- &LogMessage{
			Message: "Waiting for items to be enqueued",
			Level:   logrus.DebugLevel,
		}
		q.cond.Wait()
	}

	hostsChecked := 0
	for hostsChecked < len(q.hostOrder) {
		host := q.hostOrder[q.currentHost]
		positions := q.hostIndex[host]

		q.LoggingChan <- &LogMessage{
			Message: fmt.Sprintf("Checking host %s, positions: %d", host, len(positions)),
			Level:   logrus.DebugLevel,
		}

		if len(positions) == 0 {
			// Remove this host from the order and index
			q.hostOrder = append(q.hostOrder[:q.currentHost], q.hostOrder[q.currentHost+1:]...)
			delete(q.hostIndex, host)
			if len(q.hostOrder) == 0 {
				q.currentHost = 0
				continue
			}
			q.currentHost = q.currentHost % len(q.hostOrder)
			hostsChecked++
			continue
		}

		// We found a host with items, dequeue from here
		position := positions[0]
		q.hostIndex[host] = positions[1:]

		q.LoggingChan <- &LogMessage{
			Message: fmt.Sprintf("Dequeuing item at position %d for host %s", position, host),
			Level:   logrus.DebugLevel,
		}

		_, err := q.queueFile.Seek(int64(position), io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("failed to seek to item position: %w", err)
		}

		var item Item
		err = q.queueDecoder.Decode(&item)
		if err != nil {
			return nil, fmt.Errorf("failed to decode item: %w", err)
		}

		q.currentHost = (q.currentHost + 1) % len(q.hostOrder)

		// Update stats
		updateDequeueStats(q, host)

		err = q.saveMetadata()
		if err != nil {
			return nil, fmt.Errorf("failed to save metadata: %w", err)
		}

		return &item, nil
	}

	fmt.Println("After Loop")

	// If we've checked all hosts and found no items, loop back to wait again
	return q.Dequeue()
}
