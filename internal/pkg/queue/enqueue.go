package queue

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

// BatchEnqueue adds 1 or many items to the queue in a single operation.
// If multiple items are provided, the order in which they will be enqueued is not guaranteed.
// It WILL be less efficient than Enqueue for single items.
func (q *PersistentGroupedQueue) BatchEnqueue(items ...*Item) error {
	if !q.CanEnqueue() {
		return ErrQueueClosed
	}

	batchLen := len(items)
	if batchLen == 0 {
		return fmt.Errorf("cannot enqueue empty batch")
	}

	failedHandoverItems := []*handoverEncodedItem{}
	var once sync.Once
	onceSignalQueue := func() { q.handoverCircuitBreaker.Set(true) }
	if handover := q.handover.TryOpen(batchLen); handover {
		for _, i := range items {
			if i == nil {
				q.logger.Error("cannot enqueue nil item")
				continue
			}

			b, err := encodeItem(i)
			if err != nil {
				q.logger.Error("failed to encode item", "err", err)
				continue
			}

			item := &handoverEncodedItem{
				bytes: b,
				item:  i,
			}
			if !q.handover.TryPut(item) {
				q.logger.Error("failed to put item in handover")
				failedHandoverItems = append(failedHandoverItems, item)
			}

			once.Do(onceSignalQueue)
		}
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()

	// Seek to end of file to get start position
	startPos, err := q.queueFile.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("failed to seek to end of file: %s", err.Error())
	}

	itemsChan := make(chan *msg, len(items))

	wg := &sync.WaitGroup{}
	wg.Add(len(items))

	for _, item := range items {
		if item == nil {
			q.logger.Error("cannot enqueue nil item")
			continue
		}

		go func(i *Item, wg *sync.WaitGroup) {
			defer wg.Done()

			if q.handover.TryPut(i) {
				return
			}

			b, err := encodeItem(i)
			if err != nil {
				q.logger.Error("failed to encode item", "err", err)
				return
			}

			itemsChan <- &msg{
				bytes: b,
				item:  i,
			}
		}(item, wg)
	}

	wg.Wait()

	if leftover, exists := q.handover.TryGet(); exists {
		b, err := encodeItem(leftover)
		if err != nil {
			q.logger.Error("failed to encode item", "err", err)
		} else {
			itemsChan <- &msg{
				bytes: b,
				item:  leftover,
			}
		}
	}

	// This close IS necessary to avoid indefinitely waiting in the next loop
	close(itemsChan)

	for msg := range itemsChan {
		// Write item to file
		written, err := q.queueFile.Write(msg.bytes)
		if err != nil {
			return fmt.Errorf("failed to write item: %w", err)
		}

		// Update host index and order
		err = q.index.Add(msg.item.URL.Host, msg.item.ID, uint64(startPos), uint64(len(msg.bytes)))
		if err != nil {
			return fmt.Errorf("failed to update index: %w", err)
		}

		startPos += int64(written)

		// Update stats
		q.updateEnqueueStats(msg.item)
	}

	// Update empty status
	q.Empty.Set(false)

	return nil
}

func (q *PersistentGroupedQueue) Enqueue(item *Item) error {
	if item == nil {
		return errors.New("cannot enqueue nil item")
	}

	if !q.CanEnqueue() {
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
