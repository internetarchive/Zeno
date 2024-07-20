package queue

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"sync"
	"testing"
	"time"
)

func TestNewPersistentGroupedQueue(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	queuePath := path.Join(tempDir, "test_queue")

	q, err := NewPersistentGroupedQueue(queuePath)
	if err != nil {
		t.Fatalf("Failed to create new queue: %v", err)
	}
	defer q.Close()

	// Check if queue is properly initialized
	if q.Paused == nil {
		t.Error("Paused field not initialized")
	}
	if q.queueFile == nil {
		t.Error("queueFile not initialized")
	}
	if q.metadataFile == nil {
		t.Error("metadataFile not initialized")
	}
	if q.metadataEncoder == nil {
		t.Error("metadataEncoder not initialized")
	}
	if q.metadataDecoder == nil {
		t.Error("metadataDecoder not initialized")
	}
	if len(q.hostIndex) != 0 {
		t.Error("hostIndex not initialized as empty")
	}
	if len(q.hostOrder) != 0 {
		t.Error("hostOrder not initialized as empty")
	}
	if q.currentHost != 0 {
		t.Error("currentHost not initialized to 0")
	}
}

func TestPersistentGroupedQueue_Close(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	queuePath := path.Join(tempDir, "test_queue")

	q, err := NewPersistentGroupedQueue(queuePath)
	if err != nil {
		t.Fatalf("Failed to create new queue: %v", err)
	}

	// Test normal close
	err = q.Close()
	if err != nil {
		t.Errorf("Failed to close queue: %v", err)
	}

	// Check if files are closed
	if err := q.queueFile.Close(); err == nil {
		t.Error("queueFile not closed after Close()")
	}
	if err := q.metadataFile.Close(); err == nil {
		t.Error("metadataFile not closed after Close()")
	}

	// Test double close
	err = q.Close()
	if err != nil {
		t.Errorf("Second Close() should not return error, got: %v", err)
	}

	// Test operations after close
	_, err = q.Dequeue()
	if err != ErrQueueClosed {
		t.Errorf("Expected ErrQueueClosed on Dequeue after Close, got: %v", err)
	}

	err = q.Enqueue(&Item{})
	if err != ErrQueueClosed {
		t.Errorf("Expected ErrQueueClosed on Enqueue after Close, got: %v", err)
	}
}

func TestLargeScaleEnqueueDequeue(t *testing.T) {
	// Increase test timeout
	t.Parallel()

	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	queuePath := path.Join(tempDir, "test_queue")

	q, err := NewPersistentGroupedQueue(queuePath)
	if err != nil {
		t.Fatalf("Failed to create new queue: %v", err)
	}
	defer q.Close()

	numItems := 50000
	hosts := []string{"example.com", "test.org", "sample.net", "demo.io"}

	// Enqueue items
	startEnqueue := time.Now()
	for i := 0; i < numItems; i++ {
		host := hosts[i%len(hosts)]
		u, _ := url.Parse(fmt.Sprintf("https://%s/page%d", host, i))
		item, err := NewItem(u, nil, "page", 1, fmt.Sprintf("id-%d", i), false)
		if err != nil {
			t.Fatalf("Failed to create item %d: %v", i, err)
		}

		err = q.Enqueue(item)
		if err != nil {
			t.Fatalf("Failed to enqueue item %d: %v", i, err)
		}
	}

	// Print queue file size
	queueFile, err := os.OpenFile(path.Join(queuePath, "queue"), os.O_RDONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open queue file: %v", err)
	}

	queueFileInfo, err := queueFile.Stat()
	if err != nil {
		t.Fatalf("Failed to get queue file info: %v", err)
	}

	t.Logf("Queue file size (megabytes): %d", queueFileInfo.Size()/1024/1024)

	enqueueTime := time.Since(startEnqueue)
	t.Logf("Enqueue time for %d items: %v", numItems, enqueueTime)

	// Dequeue items
	startDequeue := time.Now()
	dequeuedItems := make(map[string]bool)
	for i := 0; i < numItems; i++ {
		item, err := q.Dequeue()
		if err != nil {
			t.Fatalf("Failed to dequeue item %d: %v", i, err)
		}
		if item == nil {
			t.Fatalf("Dequeued nil item at position %d", i)
		}
		if dequeuedItems[item.ID] {
			t.Errorf("Item with ID %s dequeued more than once", item.ID)
		}

		dequeuedItems[item.ID] = true
	}
	dequeueTime := time.Since(startDequeue)

	t.Logf("Dequeue time for %d items: %v", numItems, dequeueTime)
	t.Logf("Average enqueue time per item: %v", enqueueTime/time.Duration(numItems))
	t.Logf("Average dequeue time per item: %v", dequeueTime/time.Duration(numItems))
}

func TestParallelQueueBehavior(t *testing.T) {
	queueDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(queueDir)

	queue, err := NewPersistentGroupedQueue(queueDir)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer queue.Close()

	url1, _ := url.Parse("http://google.com/")
	url2, _ := url.Parse("http://example.com/")
	url3, _ := url.Parse("http://example.com/page")

	item1, err := NewItem(url1, nil, "page", 1, "1", false)
	if err != nil {
		t.Fatalf("Failed to create item: %v", err)
	}

	item2, err := NewItem(url2, nil, "page", 1, "2", false)
	if err != nil {
		t.Fatalf("Failed to create item: %v", err)
	}

	item3, err := NewItem(url3, nil, "page", 1, "3", false)
	if err != nil {
		t.Fatalf("Failed to create item: %v", err)
	}

	var items []*Item
	items = append(items, item1, item2, item3)

	// Enqueue the 3 items in parallel
	wg := sync.WaitGroup{}
	wg.Add(3)

	errCh := make(chan error, 3)

	for i, item := range items {
		go func(j int, item *Item) {
			defer wg.Done()

			err := queue.Enqueue(item)
			if err != nil {
				errCh <- fmt.Errorf("Failed to enqueue item %d: %v", j, err)
			}
		}(i, item)
	}

	wg.Wait()
	close(errCh)

	// Check for enqueue errors
	for err := range errCh {
		t.Fatalf("%v", err)
	}

	// Then dequeue them sequentially, it should not give the 2 example.com items in a row
	for i := 0; i < 3; i++ {
		dequeued, err := queue.Dequeue()
		if err != nil {
			t.Fatalf("Failed to dequeue item: %v", err)
		}

		if dequeued == nil {
			t.Fatalf("Dequeued nil item at position %d", i)
		}
	}

	// Queue back 100 items, then dequeue them in parallel
	// We have 2 different hosts and make 100 random URLs from them, so we should not get 2 items from the same host in a row
	numItems := 100
	hosts := []string{"example.com", "example.org"}

	wg.Add(numItems)

	errCh = make(chan error, numItems)
	for i := 0; i < numItems; i++ {
		host := hosts[i%len(hosts)]

		u, _ := url.Parse(fmt.Sprintf("http://%s/page%d", host, i))
		item, err := NewItem(u, nil, "page", 1, fmt.Sprintf("id-%d", i), false)
		if err != nil {
			t.Fatalf("Failed to create item %d: %v", i, err)
		}

		go func(j int, item *Item) {
			defer wg.Done()

			err := queue.Enqueue(item)
			if err != nil {
				errCh <- fmt.Errorf("Failed to enqueue item %d: %v", j, err)
			}
		}(i, item)
	}

	wg.Wait()
	close(errCh)

	// Check for enqueue errors
	for err := range errCh {
		t.Fatalf("%v", err)
	}

	wg.Add(numItems)

	errCh = make(chan error, numItems)
	for i := 0; i < numItems; i++ {
		go func(j int) {
			defer wg.Done()

			dequeued, err := queue.Dequeue()
			if err != nil {
				errCh <- err
				return
			}

			if dequeued == nil {
				errCh <- fmt.Errorf("Dequeued nil item at position %d", j)
				return
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	// Check for dequeue errors
	for err := range errCh {
		t.Fatalf("%v", err)
	}

	wg.Wait()
}
