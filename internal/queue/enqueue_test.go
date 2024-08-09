package queue

import (
	"net/url"
	"os"
	"path"
	"testing"
	"time"

	"github.com/internetarchive/Zeno/internal/stats"
)

func TestEnqueue(t *testing.T) {
	t.Run("Enqueue single item", func(t *testing.T) {
		stats.Reset()
		stats.Init(nil)
		tempDir, err := os.MkdirTemp("", "queue_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		q, err := NewPersistentGroupedQueue(path.Join(tempDir, "test_queue"), false, false)
		if err != nil {
			t.Fatalf("Failed to create new queue: %v", err)
		}
		defer q.Close()

		url, _ := url.Parse("http://example.fr")
		item, err := NewItem(url, nil, "test", 0, "", false)
		if err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}

		err = q.Enqueue(item)
		if err != nil {
			t.Fatalf("Failed to enqueue item: %v", err)
		}

		if totalElem := stats.GetQueueTotalElementsCount(); totalElem != 1 {
			t.Fatalf("Expected TotalElements to be 1, got %d", totalElem)
		}

		if uniqueHosts := stats.GetQueueUniqueHostsCount(); uniqueHosts != 1 {
			t.Fatalf("Expected UniqueHosts to be 1, got %d", uniqueHosts)
		}

		elementsPerHost := stats.GetElementsPerHost()

		if elementsPerHost["example.fr"] != 1 {
			t.Fatalf("Expected ElementsPerHost[example.fr] to be 1, got %d", elementsPerHost["example.fr"])
		}
	})

	t.Run("Enqueue multiple items", func(t *testing.T) {
		stats.Reset()
		stats.Init(nil)
		tempDir, err := os.MkdirTemp("", "queue_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		q, err := NewPersistentGroupedQueue(path.Join(tempDir, "test_queue"), false, false)
		if err != nil {
			t.Fatalf("Failed to create new queue: %v", err)
		}
		defer q.Close()

		hosts := []string{"example.org", "example.net", "example.fr", "example.fr"}

		for _, host := range hosts {
			url, _ := url.Parse("http://" + host)
			item, err := NewItem(url, nil, "test", 0, "", false)
			if err != nil {
				t.Fatalf("Failed to create item for host %s: %v", host, err)
			}

			err = q.Enqueue(item)
			if err != nil {
				t.Fatalf("Failed to enqueue item for host %s: %v", host, err)
			}
		}

		if totalElem := stats.GetQueueTotalElementsCount(); totalElem != 4 {
			t.Fatalf("Expected TotalElements to be 4, got %d", totalElem)
		}

		if uniqueHosts := stats.GetQueueUniqueHostsCount(); uniqueHosts != 3 {
			t.Fatalf("Expected UniqueHosts to be 3, got %d", uniqueHosts)
		}

		elementsPerHost := stats.GetElementsPerHost()

		if elementsPerHost["example.fr"] != 2 {
			t.Fatalf("Expected ElementsPerHost[example.fr] to be 2, got %d", elementsPerHost["example.fr"])
		}

		if q.Empty.Get() {
			t.Fatal("Expected queue to be non-empty")
		}
	})

	t.Run("Enqueue to closed queue", func(t *testing.T) {
		stats.Reset()
		stats.Init(nil)
		tempDir, err := os.MkdirTemp("", "queue_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		q, err := NewPersistentGroupedQueue(path.Join(tempDir, "test_queue"), false, false)
		if err != nil {
			t.Fatalf("Failed to create new queue: %v", err)
		}

		q.Close()

		url, _ := url.Parse("http://closed.com")
		item, err := NewItem(url, nil, "test", 0, "", false)
		if err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}

		err = q.Enqueue(item)
		if err != ErrQueueClosed {
			t.Fatalf("Expected ErrQueueClosed, got: %v", err)
		}
	})

	t.Run("Check enqueue times", func(t *testing.T) {
		stats.Reset()
		stats.Init(nil)
		tempDir, err := os.MkdirTemp("", "queue_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		q, err := NewPersistentGroupedQueue(path.Join(tempDir, "test_queue"), false, false)
		if err != nil {
			t.Fatalf("Failed to create new queue: %v", err)
		}
		defer q.Close()

		url, _ := url.Parse("http://timetest.com")
		item, err := NewItem(url, nil, "test", 0, "", false)
		if err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}

		err = q.Enqueue(item)
		if err != nil {
			t.Fatalf("Failed to enqueue item: %v", err)
		}

		if stats.GetFirstEnqueueTime().IsZero() {
			t.Fatal("FirstEnqueueTime should not be zero")
		}
		if stats.GetLastEnqueueTime().IsZero() {
			t.Fatal("LastEnqueueTime should not be zero")
		}
		if enqueueCount := stats.GetEnqueueCount(); enqueueCount != 1 {
			t.Fatalf("Expected EnqueueCount to be 1, got %d", enqueueCount)
		}

		time.Sleep(10 * time.Millisecond)
		err = q.Enqueue(item)
		if err != nil {
			t.Fatalf("Failed to enqueue item: %v", err)
		}

		if !stats.GetLastEnqueueTime().After(stats.GetFirstEnqueueTime()) {
			t.Fatal("LastEnqueueTime should be after FirstEnqueueTime")
		}
		if enqueueCount := stats.GetEnqueueCount(); enqueueCount != 2 {
			t.Fatalf("Expected EnqueueCount to be 2, got %d", enqueueCount)
		}

		if q.Empty.Get() {
			t.Fatal("Expected queue to be non-empty")
		}
	})

	t.Run("Check host order", func(t *testing.T) {
		stats.Reset()
		stats.Init(nil)
		tempDir, err := os.MkdirTemp("", "queue_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		q, err := NewPersistentGroupedQueue(path.Join(tempDir, "test_queue"), false, false)
		if err != nil {
			t.Fatalf("Failed to create new queue: %v", err)
		}
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
				t.Fatalf("Failed to enqueue item: %v", err)
			}

		}

		if len(q.index.GetHosts()) != 3 {
			t.Fatalf("Expected hostOrder length to be 3, got %d", len(q.index.GetHosts()))
		}
		for i, host := range hosts {
			if i < len(q.index.GetHosts()) {
				if q.index.GetHosts()[i] != host {
					t.Fatalf("Expected hostOrder[%d] to be %s, got %s", i, host, q.index.GetHosts()[i])
				}
			} else {
				t.Fatalf("hostOrder is shorter than expected, missing %s", host)
			}
		}
	})
}

func TestBatchEnqueue(t *testing.T) {
	t.Run("Enqueue single item", func(t *testing.T) {
		stats.Reset()
		stats.Init(nil)
		tempDir, err := os.MkdirTemp("", "queue_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		q, err := NewPersistentGroupedQueue(path.Join(tempDir, "test_queue"), false, false)
		if err != nil {
			t.Fatalf("Failed to create new queue: %v", err)
		}
		defer q.Close()

		url, _ := url.Parse("http://example.fr")
		item, err := NewItem(url, nil, "test", 0, "", false)
		if err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}

		err = q.BatchEnqueue(item)
		if err != nil {
			t.Fatalf("Failed to enqueue item: %v", err)
		}

		if totalElem := stats.GetQueueTotalElementsCount(); totalElem != 1 {
			t.Fatalf("Expected TotalElements to be 1, got %d", totalElem)
		}

		if uniqueHosts := stats.GetQueueUniqueHostsCount(); uniqueHosts != 1 {
			t.Fatalf("Expected UniqueHosts to be 1, got %d", stats.GetQueueUniqueHostsCount())
		}

		elementsPerHost := stats.GetElementsPerHost()

		if elementsPerHost["example.fr"] != 1 {
			t.Fatalf("Expected ElementsPerHost[example.fr] to be 1, got %d", elementsPerHost["example.fr"])
		}

		if q.Empty.Get() {
			t.Fatal("Expected queue to be non-empty")
		}
	})

	t.Run("Enqueue multiple items", func(t *testing.T) {
		stats.Reset()
		stats.Init(nil)
		tempDir, err := os.MkdirTemp("", "queue_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		q, err := NewPersistentGroupedQueue(path.Join(tempDir, "test_queue"), false, false)
		if err != nil {
			t.Fatalf("Failed to create new queue: %v", err)
		}
		defer q.Close()

		hosts := []string{"example.org", "example.net", "example.fr", "example.fr"}
		items := make([]*Item, 0, len(hosts))

		for _, host := range hosts {
			url, _ := url.Parse("http://" + host)
			item, err := NewItem(url, nil, "test", 0, "", false)
			if err != nil {
				t.Fatalf("Failed to create item for host %s: %v", host, err)
			}

			items = append(items, item)
		}

		err = q.BatchEnqueue(items...)
		if err != nil {
			t.Fatalf("Failed to enqueue items: %v", err)
		}

		if totalElem := stats.GetQueueTotalElementsCount(); totalElem != 4 {
			t.Fatalf("Expected TotalElements to be 4, got %d", totalElem)
		}

		if uniqueHosts := stats.GetQueueUniqueHostsCount(); uniqueHosts != 3 {
			t.Fatalf("Expected UniqueHosts to be 3, got %d", stats.GetQueueUniqueHostsCount())
		}

		elementsPerHost := stats.GetElementsPerHost()

		if elementsPerHost["example.fr"] != 2 {
			t.Fatalf("Expected ElementsPerHost[example.fr] to be 2, got %d", elementsPerHost["example.fr"])
		}

		if q.Empty.Get() {
			t.Fatal("Expected queue to be non-empty")
		}
	})

	t.Run("Enqueue to closed queue", func(t *testing.T) {
		stats.Reset()
		stats.Init(nil)
		tempDir, err := os.MkdirTemp("", "queue_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		q, err := NewPersistentGroupedQueue(path.Join(tempDir, "test_queue"), false, false)
		if err != nil {
			t.Fatalf("Failed to create new queue: %v", err)
		}
		q.Close()

		url, _ := url.Parse("http://closed.com")
		item, err := NewItem(url, nil, "test", 0, "", false)
		if err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}

		err = q.BatchEnqueue(item)
		if err != ErrQueueClosed {
			t.Fatalf("Expected ErrQueueClosed, got: %v", err)
		}
	})

	t.Run("Check enqueue times", func(t *testing.T) {
		stats.Reset()
		stats.Init(nil)
		tempDir, err := os.MkdirTemp("", "queue_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		q, err := NewPersistentGroupedQueue(path.Join(tempDir, "test_queue"), false, false)
		if err != nil {
			t.Fatalf("Failed to create new queue: %v", err)
		}
		defer q.Close()

		url, _ := url.Parse("http://timetest.com")
		item, err := NewItem(url, nil, "test", 0, "", false)
		if err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}

		err = q.BatchEnqueue(item)
		if err != nil {
			t.Fatalf("Failed to enqueue item: %v", err)
		}

		if stats.GetFirstEnqueueTime().IsZero() {
			t.Fatal("FirstEnqueueTime should not be zero")
		}

		if stats.GetLastEnqueueTime().IsZero() {
			t.Fatal("LastEnqueueTime should not be zero")
		}

		if enqueueCount := stats.GetEnqueueCount(); enqueueCount != 1 {
			t.Fatalf("Expected EnqueueCount to be 1, got %d", enqueueCount)
		}

		time.Sleep(10 * time.Millisecond)
		err = q.BatchEnqueue(item)
		if err != nil {
			t.Fatalf("Failed to enqueue item: %v", err)
		}

		if !stats.GetLastEnqueueTime().After(stats.GetFirstEnqueueTime()) {
			t.Fatal("LastEnqueueTime should be after FirstEnqueueTime")
		}

		if enqueueCount := stats.GetEnqueueCount(); enqueueCount != 2 {
			t.Fatalf("Expected EnqueueCount to be 2, got %d", enqueueCount)
		}

		if q.Empty.Get() {
			t.Fatal("Expected queue to be non-empty")
		}
	})

	t.Run("Check host order", func(t *testing.T) {
		stats.Reset()
		stats.Init(nil)
		tempDir, err := os.MkdirTemp("", "queue_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		q, err := NewPersistentGroupedQueue(path.Join(tempDir, "test_queue"), false, false)
		if err != nil {
			t.Fatalf("Failed to create new queue: %v", err)
		}
		defer q.Close()

		hosts := []string{"first.com", "second.com", "third.com"}
		for _, host := range hosts {
			url, _ := url.Parse("http://" + host)
			item, err := NewItem(url, nil, "test", 0, "", false)
			if err != nil {
				t.Fatalf("Failed to create item: %v", err)
			}

			err = q.BatchEnqueue(item)
			if err != nil {
				t.Fatalf("Failed to enqueue item: %v", err)
			}
		}

		if len(q.index.GetHosts()) != 3 {
			t.Fatalf("Expected hostOrder length to be 3, got %d", len(q.index.GetHosts()))
		}

		for i, host := range hosts {
			if i < len(q.index.GetHosts()) {
				if q.index.GetHosts()[i] != host {
					t.Fatalf("Expected hostOrder[%d] to be %s, got %s", i, host, q.index.GetHosts()[i])
				}
			} else {
				t.Fatalf("hostOrder is shorter than expected, missing %s", host)
			}
		}

		if q.Empty.Get() {
			t.Fatal("Expected queue to be non-empty")
		}
	})
}
