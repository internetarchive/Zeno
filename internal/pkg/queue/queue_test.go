package queue

import (
	"net/url"
	"os"
	"path"
	"testing"
)

func TestNewItem(t *testing.T) {
	// Test cases
	testCases := []struct {
		name             string
		url              string
		parentItem       *Item
		itemType         string
		hop              uint8
		id               string
		bypassSeencheck  bool
		expectedHostname string
	}{
		{
			name:             "Basic URL",
			url:              "https://example.com/page",
			parentItem:       nil,
			itemType:         "page",
			hop:              1,
			id:               "",
			bypassSeencheck:  false,
			expectedHostname: "example.com",
		},
		{
			name:             "URL with ID and BypassSeencheck",
			url:              "https://test.org/resource",
			parentItem:       &Item{URL: &url.URL{Host: "parent.com"}},
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
			item := NewItem(parsedURL, tc.parentItem, tc.itemType, tc.hop, tc.id, tc.bypassSeencheck)

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
			if item.ParentItem != tc.parentItem {
				t.Errorf("Expected parent item %v, got %v", tc.parentItem, item.ParentItem)
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

			expectedBypassSeencheck := "false"
			if tc.bypassSeencheck {
				expectedBypassSeencheck = "true"
			}
			if item.BypassSeencheck != expectedBypassSeencheck {
				t.Errorf("Expected BypassSeencheck %s, got %s", expectedBypassSeencheck, item.BypassSeencheck)
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
	if q.queueEncoder == nil {
		t.Error("queueEncoder not initialized")
	}
	if q.queueDecoder == nil {
		t.Error("queueDecoder not initialized")
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
