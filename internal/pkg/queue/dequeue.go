package queue

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"google.golang.org/protobuf/proto"
)

// Dequeue removes and returns the next item from the queue
// It blocks until an item is available
func (q *PersistentGroupedQueue) Dequeue() (*Item, error) {
	if q.closed {
		return nil, ErrQueueClosed
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()

	for {
		q.hostMutex.Lock()
		if len(q.hostOrder) == 0 {
			q.hostMutex.Unlock()
			q.mutex.Unlock()
			q.cond.Wait()
			q.mutex.Lock()
			continue
		}

		host := q.hostOrder[0]
		positions := q.hostIndex[host]

		if len(positions) == 0 {
			delete(q.hostIndex, host)
			q.hostOrder = q.hostOrder[1:]
			q.hostMutex.Unlock()
			continue
		}

		// Remove the 2 elements we are going to use
		// (position and size of the item)
		q.hostIndex[host] = positions[2:]

		if len(q.hostIndex[host]) == 0 {
			delete(q.hostIndex, host)
			q.hostOrder = q.hostOrder[1:]
		} else {
			q.hostOrder = append(q.hostOrder[1:], host)
		}

		q.hostMutex.Unlock()

		// Read and unmarshal the item
		itemBytes, err := q.ReadItemAt(positions[0], positions[1])
		if err != nil {
			return nil, fmt.Errorf("failed to read item at position %d: %w", positions[0], err)
		}

		protoItem := &ProtoItem{}
		err = proto.Unmarshal(itemBytes, protoItem)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal item: %w", err)
		}

		var parsedURL url.URL
		err = json.Unmarshal(protoItem.Url, &parsedURL)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal URL: %w", err)
		}

		item := &Item{
			ProtoItem: protoItem,
			URL:       &parsedURL,
		}

		err = item.UnmarshalParent()
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal parent item: %w", err)
		}

		updateDequeueStats(q, item.Host)

		return item, nil
	}
}

func (q *PersistentGroupedQueue) ReadItemAt(position uint64, itemSize uint64) ([]byte, error) {
	// Ensure the file pointer is at the correct position
	_, err := q.queueFile.Seek(int64(position), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to item position: %w", err)
	}

	// Read the specific number of bytes for the item
	itemBytes := make([]byte, itemSize)
	_, err = io.ReadFull(q.queueFile, itemBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read item bytes: %w", err)
	}

	return itemBytes, nil
}
