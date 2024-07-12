package queue

import (
	"fmt"
	"net/url"
	"os"
	"testing"
)

func TestDequeue(t *testing.T) {
	// Create a temporary directory for the queue files
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a new queue
	q, err := NewPersistentGroupedQueue(tempDir, nil)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer q.Close()

	// Test dequeue on empty queue
	_, err = q.Dequeue()
	if err != ErrQueueTimeout {
		t.Errorf("Expected ErrQueueTimeout, got %v", err)
	}

	// Enqueue some items
	url1, _ := url.Parse("http://example1.com")
	url2, _ := url.Parse("http://example2.com")
	url3, _ := url.Parse("http://example1.com/page")

	items := []*Item{
		NewItem(url1, nil, "test", 0, "1", false),
		NewItem(url2, nil, "test", 0, "2", false),
		NewItem(url3, nil, "test", 0, "3", false),
	}

	for _, item := range items {
		err = q.Enqueue(item)
		if err != nil {
			t.Fatalf("Failed to enqueue item: %v", err)
		}
	}

	// Test dequeue
	for i := 0; i < 3; i++ {
		dequeued, err := q.Dequeue()
		if err != nil {
			t.Fatalf("Failed to dequeue item: %v", err)
		}
		if dequeued == nil {
			t.Fatalf("Dequeued item is nil")
		}
		// Check if the dequeued item matches one of the enqueued items
		found := false
		for _, item := range items {
			if dequeued.ID == item.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Dequeued unexpected item: %v", dequeued)
		}
	}

	// Test dequeue on empty queue again
	_, err = q.Dequeue()
	if err != ErrQueueTimeout {
		t.Errorf("Expected ErrQueueTimeout, got %v", err)
	}

	// Test dequeue on closed queue
	q.Close()
	_, err = q.Dequeue()
	if err != ErrQueueClosed {
		t.Errorf("Expected ErrQueueClosed, got %v", err)
	}
}

func TestDequeueHostOrder(t *testing.T) {
	// Create a temporary directory for the queue files
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a new queue
	q, err := NewPersistentGroupedQueue(tempDir, nil)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer q.Close()

	// Enqueue items from different hosts
	urls := []string{
		"http://example1.com/1",
		"http://example2.com/1",
		"http://example3.com/1",
		"http://example1.com/2",
		"http://example2.com/2",
		"http://example3.com/2",
	}

	for i, urlStr := range urls {
		url, _ := url.Parse(urlStr)
		item := NewItem(url, nil, "test", 0, fmt.Sprint(i), false)
		err = q.Enqueue(item)
		if err != nil {
			t.Fatalf("Failed to enqueue item: %v", err)
		}
	}

	// Dequeue and check host order
	expectedHosts := []string{
		"example1.com",
		"example2.com",
		"example3.com",
		"example1.com",
		"example2.com",
		"example3.com",
	}

	for i, expectedHost := range expectedHosts {
		dequeued, err := q.Dequeue()
		if err != nil {
			t.Fatalf("Failed to dequeue item: %v", err)
		}
		if dequeued.Host != expectedHost {
			t.Errorf("Expected host %s for dequeue %d, got %s", expectedHost, i, dequeued.Host)
		}
	}
}
