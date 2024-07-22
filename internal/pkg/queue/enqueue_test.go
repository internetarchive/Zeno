package queue

import (
	"net/url"
	"os"
	"path"
	"testing"
	"time"
)

func TestEnqueue(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	q, err := NewPersistentGroupedQueue(path.Join(tempDir, "test_queue"), nil)
	if err != nil {
		t.Fatalf("Failed to create new queue: %v", err)
	}
	defer q.Close()

	t.Run("Enqueue single item", func(t *testing.T) {
		url, _ := url.Parse("http://example.com")
		item, err := NewItem(url, nil, "test", 0, "", false)
		if err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}

		err = q.Enqueue(item)
		if err != nil {
			t.Errorf("Failed to enqueue item: %v", err)
		}

		if q.stats.TotalElements != 1 {
			t.Errorf("Expected TotalElements to be 1, got %d", q.stats.TotalElements)
		}
		if q.stats.UniqueHosts != 1 {
			t.Errorf("Expected UniqueHosts to be 1, got %d", q.stats.UniqueHosts)
		}
		if q.stats.ElementsPerHost["example.com"] != 1 {
			t.Errorf("Expected ElementsPerHost[example.com] to be 1, got %d", q.stats.ElementsPerHost["example.com"])
		}
	})

	t.Run("Enqueue multiple items", func(t *testing.T) {
		hosts := []string{"example.org", "example.net", "example.com"}
		for _, host := range hosts {
			url, _ := url.Parse("http://" + host)
			item, err := NewItem(url, nil, "test", 0, "", false)
			if err != nil {
				t.Fatalf("Failed to create item for host %s: %v", host, err)
			}

			err = q.Enqueue(item)
			if err != nil {
				t.Errorf("Failed to enqueue item for host %s: %v", host, err)
			}
		}

		if q.stats.TotalElements != 4 {
			t.Errorf("Expected TotalElements to be 4, got %d", q.stats.TotalElements)
		}
		if q.stats.UniqueHosts != 3 {
			t.Errorf("Expected UniqueHosts to be 3, got %d", q.stats.UniqueHosts)
		}
		if q.stats.ElementsPerHost["example.com"] != 2 {
			t.Errorf("Expected ElementsPerHost[example.com] to be 2, got %d", q.stats.ElementsPerHost["example.com"])
		}
	})

	t.Run("Enqueue to closed queue", func(t *testing.T) {
		q.Close()
		url, _ := url.Parse("http://closed.com")
		item, err := NewItem(url, nil, "test", 0, "", false)
		if err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}

		err = q.Enqueue(item)
		if err != ErrQueueClosed {
			t.Errorf("Expected ErrQueueClosed, got: %v", err)
		}
	})

	t.Run("Check enqueue times", func(t *testing.T) {
		q, _ := NewPersistentGroupedQueue(path.Join(tempDir, "time_test_queue"), nil)
		defer q.Close()

		url, _ := url.Parse("http://timetest.com")
		item, err := NewItem(url, nil, "test", 0, "", false)
		if err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}

		err = q.Enqueue(item)
		if err != nil {
			t.Errorf("Failed to enqueue item: %v", err)
		}

		if q.stats.FirstEnqueueTime.IsZero() {
			t.Error("FirstEnqueueTime should not be zero")
		}
		if q.stats.LastEnqueueTime.IsZero() {
			t.Error("LastEnqueueTime should not be zero")
		}
		if q.stats.EnqueueCount != 1 {
			t.Errorf("Expected EnqueueCount to be 1, got %d", q.stats.EnqueueCount)
		}

		time.Sleep(10 * time.Millisecond)
		err = q.Enqueue(item)
		if err != nil {
			t.Errorf("Failed to enqueue item: %v", err)
		}

		if !q.stats.LastEnqueueTime.After(q.stats.FirstEnqueueTime) {
			t.Error("LastEnqueueTime should be after FirstEnqueueTime")
		}
		if q.stats.EnqueueCount != 2 {
			t.Errorf("Expected EnqueueCount to be 2, got %d", q.stats.EnqueueCount)
		}

	})

	t.Run("Check host order", func(t *testing.T) {
		q, _ := NewPersistentGroupedQueue(path.Join(tempDir, "order_test_queue"), nil)
		defer q.Close()

		hosts := []string{"first.com", "second.com", "third.com"}
		for _, host := range hosts {
			url, _ := url.Parse("http://" + host)
			item, err := NewItem(url, nil, "test", 0, "", false)
			if err != nil {
				t.Fatalf("Failed to create item: %v", err)
			}

			err = q.Enqueue(item)
			if err != nil {
				t.Errorf("Failed to enqueue item: %v", err)
			}

		}

		if len(q.index.GetHosts()) != 3 {
			t.Errorf("Expected hostOrder length to be 3, got %d", len(q.index.GetHosts()))
		}
		for i, host := range hosts {
			if i < len(q.index.GetHosts()) {
				if q.index.GetHosts()[i] != host {
					t.Errorf("Expected hostOrder[%d] to be %s, got %s", i, host, q.index.GetHosts()[i])
				}
			} else {
				t.Errorf("hostOrder is shorter than expected, missing %s", host)
			}
		}
	})
}
