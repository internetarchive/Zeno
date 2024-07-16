package queue

import (
	"fmt"
	"net/url"
	"os"
	"path"
	sync "sync"
	"testing"
	"time"
)

func TestNewItem(t *testing.T) {
	// Test cases
	testCases := []struct {
		name             string
		url              string
		parent           *Item
		itemType         string
		hop              uint64
		id               string
		bypassSeencheck  bool
		expectedHostname string
	}{
		{
			name:             "Basic URL",
			url:              "https://example.com/page",
			parent:           nil,
			itemType:         "page",
			hop:              1,
			id:               "",
			bypassSeencheck:  false,
			expectedHostname: "example.com",
		},
		{
			name:             "URL with ID and BypassSeencheck",
			url:              "https://test.org/resource",
			parent:           &Item{URL: &url.URL{Host: "parent.com"}},
			itemType:         "resource",
			hop:              2,
			id:               "custom-id",
			bypassSeencheck:  true,
			expectedHostname: "test.org",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse URL
			parsedURL, err := url.Parse(tc.url)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			// Create new item
			item, err := NewItem(parsedURL, tc.parent, tc.itemType, tc.hop, tc.id, tc.bypassSeencheck)
			if err != nil {
				t.Fatalf("Failed to create new item: %v", err)
			}

			// Assertions
			if item.URL != parsedURL {
				t.Errorf("Expected URL %v, got %v", parsedURL, item.URL)
			}
			if item.Host != tc.expectedHostname {
				t.Errorf("Expected host %s, got %s", tc.expectedHostname, item.Host)
			}
			if item.Hop != tc.hop {
				t.Errorf("Expected hop %d, got %d", tc.hop, item.Hop)
			}
			if item.Parent != tc.parent {
				t.Errorf("Expected parent item %v, got %v", tc.parent, item.ParentItem)
			}
			if item.Type != tc.itemType {
				t.Errorf("Expected item type %s, got %s", tc.itemType, item.Type)
			}
			if tc.id != "" {
				if item.ID != tc.id {
					t.Errorf("Expected ID %s, got %s", tc.id, item.ID)
				}
			} else {
				if item.ID != "" {
					t.Errorf("Expected empty ID, got %s", item.ID)
				}
			}

			expectedBypassSeencheck := false
			if tc.bypassSeencheck {
				expectedBypassSeencheck = true
			}
			if item.BypassSeencheck != expectedBypassSeencheck {
				t.Errorf("Expected BypassSeencheck %t, got %t", expectedBypassSeencheck, item.BypassSeencheck)
			}

			// Check that Hash is not empty (we can't predict its exact value)
			if item.Hash == 0 {
				t.Error("Expected non-zero Hash")
			}
		})
	}
}

func TestNewPersistentGroupedQueue(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	queuePath := path.Join(tempDir, "test_queue")
	loggingChan := make(chan *LogMessage, 100)

	q, err := NewPersistentGroupedQueue(queuePath, loggingChan)
	if err != nil {
		t.Fatalf("Failed to create new queue: %v", err)
	}
	defer q.Close()

	// Check if queue is properly initialized
	if q.Paused == nil {
		t.Error("Paused field not initialized")
	}
	if q.LoggingChan != loggingChan {
		t.Error("LoggingChan not set correctly")
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
	loggingChan := make(chan *LogMessage, 100)

	q, err := NewPersistentGroupedQueue(queuePath, loggingChan)
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
	loggingChan := make(chan *LogMessage, 1000000)

	q, err := NewPersistentGroupedQueue(queuePath, loggingChan)
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

func TestParallelEnqueueDequeue(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	queuePath := path.Join(tempDir, "test_queue")
	loggingChan := make(chan *LogMessage, 100)

	q, err := NewPersistentGroupedQueue(queuePath, loggingChan)
	if err != nil {
		t.Fatalf("Failed to create new queue: %v", err)
	}
	defer q.Close()

	const (
		numWorkers = 10
		numItems   = 1000
	)

	var wg sync.WaitGroup
	wg.Add(numWorkers * 2) // For both enqueuers and dequeuers

	// Start enqueuers
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numItems; j++ {
				urlStr := fmt.Sprintf("http://example.com/%d/%d", workerID, j)

				u, err := url.Parse(urlStr)
				if err != nil {
					t.Errorf("Failed to parse URL: %v", err)
					continue
				}

				item, err := NewItem(u, nil, "page", 1, fmt.Sprintf("id-%d", i), false)
				if err != nil {
					t.Errorf("Failed to create item %d: %v", i, err)
					continue
				}

				err = q.Enqueue(item)
				if err != nil {
					t.Errorf("Failed to enqueue item: %v", err)
				}
			}
		}(i)
	}

	// Start dequeuers
	dequeued := make(chan *Item, numWorkers*numItems)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for {
				item, err := q.Dequeue()
				if err == ErrQueueEmpty {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				if err != nil {
					t.Errorf("Failed to dequeue item: %v", err)
					return
				}
				dequeued <- item
				if len(dequeued) == numWorkers*numItems {
					return
				}
			}
		}()
	}

	wg.Wait()
	close(dequeued)

	if len(dequeued) != numWorkers*numItems {
		t.Errorf("Expected %d items, got %d", numWorkers*numItems, len(dequeued))
	}
}
