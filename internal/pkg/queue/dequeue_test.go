package queue

import (
	"net/url"
	"os"
	"path"
	"sync"
	"testing"
	"time"
)

func TestPersistentGroupedQueue_dequeue(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	q, err := NewPersistentGroupedQueue(path.Join(tempDir, "test_queue"), make(chan *LogMessage, 100))
	if err != nil {
		t.Fatalf("Failed to create new queue: %v", err)
	}
	defer q.Close()

	// Test dequeue from empty queue
	_, err = q.dequeue()
	if err != ErrQueueEmpty {
		t.Errorf("Expected ErrQueueEmpty, got: %v", err)
	}

	// Add items to the queue
	testItems := []*Item{
		{URL: &url.URL{Host: "example.com"}},
		{URL: &url.URL{Host: "example.org"}},
		{URL: &url.URL{Host: "example.net"}},
		{URL: &url.URL{Host: "example.com"}},
	}
	for _, item := range testItems {
		err = q.Enqueue(item)
		if err != nil {
			t.Fatalf("Failed to enqueue item: %v", err)
		}
	}

	// Verify queue state after enqueue
	t.Logf("Queue state after enqueue: %+v", q)

	// Test dequeue order and host rotation
	expectedOrder := []string{"example.com", "example.org", "example.net", "example.com"}
	for i, expectedHost := range expectedOrder {
		item, err := q.dequeue()
		if err != nil {
			t.Errorf("Unexpected error on dequeue %d: %v", i, err)
			continue
		}
		if item == nil {
			t.Errorf("Dequeued item %d is nil", i)
			continue
		}
		if item.URL == nil {
			t.Errorf("Dequeued item %d has nil URL", i)
			continue
		}
		if item.URL.Host != expectedHost {
			t.Errorf("Expected host %s, got %s", expectedHost, item.URL.Host)
		}
		t.Logf("Successfully dequeued item %d: %s", i, item.URL.Host)
	}

	// Test queue becomes empty
	_, err = q.dequeue()
	if err != ErrQueueEmpty {
		t.Errorf("Expected ErrQueueEmpty, got: %v", err)
	}
}

func TestPersistentGroupedQueue_Dequeue(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	q, err := NewPersistentGroupedQueue(path.Join(tempDir, "test_queue"), make(chan *LogMessage, 100))
	if err != nil {
		t.Fatalf("Failed to create new queue: %v", err)
	}
	defer q.Close()

	// Test basic dequeue operation
	err = q.Enqueue(&Item{URL: &url.URL{Host: "example.com"}})
	if err != nil {
		t.Fatalf("Failed to enqueue item: %v", err)
	}

	item, err := q.Dequeue()
	if err != nil {
		t.Errorf("Unexpected error on dequeue: %v", err)
	}
	if item.URL.Host != "example.com" {
		t.Errorf("Expected host example.com, got %s", item.URL.Host)
	}

	// Test concurrent access
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := q.Enqueue(&Item{URL: &url.URL{Host: "concurrent.com"}})
			if err != nil {
				t.Errorf("Failed to enqueue in goroutine: %v", err)
			}
		}()
	}
	wg.Wait()

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := q.Dequeue()
			if err != nil {
				t.Errorf("Failed to dequeue in goroutine: %v", err)
			}
		}()
	}
	wg.Wait()

	// Test timeout behavior
	timeoutStart := time.Now()
	_, err = q.Dequeue()
	if err != ErrQueueTimeout {
		t.Errorf("Expected ErrQueueTimeout, got: %v", err)
	}
	if time.Since(timeoutStart) < 5*time.Second {
		t.Errorf("Dequeue returned before timeout period")
	}

	// Test behavior when queue is closed
	q.Close()
	_, err = q.Dequeue()
	if err != ErrQueueClosed {
		t.Errorf("Expected ErrQueueClosed, got: %v", err)
	}
}
