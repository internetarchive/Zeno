package index

import (
	"encoding/gob"
	"fmt"
	"os"
	"path"
	"strconv"
	"testing"
	"time"
)

func provideTestIndexManager(t *testing.T) (*IndexManager, string) {
	queueDir, err := os.MkdirTemp("", "index_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	walPath := path.Join(queueDir, "/index_wal")
	indexPath := path.Join(queueDir, "/index")

	walFile, err := os.OpenFile(walPath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("failed to open WAL file: %v", err)
	}

	indexFile, err := os.OpenFile(indexPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		walFile.Close()
		t.Fatalf("failed to open index file: %v", err)
	}

	im := &IndexManager{
		hostIndex:    newIndex(),
		walFile:      walFile,
		indexFile:    indexFile,
		walEncoder:   gob.NewEncoder(walFile),
		walDecoder:   gob.NewDecoder(walFile),
		indexEncoder: gob.NewEncoder(indexFile),
		indexDecoder: gob.NewDecoder(indexFile),
		dumpTicker:   time.NewTicker(time.Duration(dumpFrequency) * time.Second),
		lastDumpTime: time.Now(),
	}

	return im, queueDir
}

func provideBenchmarkIndexManager(b *testing.B) (*IndexManager, string) {
	queueDir, err := os.MkdirTemp("", "index_test")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}

	walPath := path.Join(queueDir, "/index_wal")
	indexPath := path.Join(queueDir, "/index")

	walFile, err := os.OpenFile(walPath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		b.Fatalf("failed to open WAL file: %v", err)
	}

	indexFile, err := os.OpenFile(indexPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		walFile.Close()
		b.Fatalf("failed to open index file: %v", err)
	}

	im := &IndexManager{
		hostIndex:    newIndex(),
		walFile:      walFile,
		indexFile:    indexFile,
		walEncoder:   gob.NewEncoder(walFile),
		walDecoder:   gob.NewDecoder(walFile),
		indexEncoder: gob.NewEncoder(indexFile),
		indexDecoder: gob.NewDecoder(indexFile),
		dumpTicker:   time.NewTicker(time.Duration(dumpFrequency) * time.Second),
		lastDumpTime: time.Now(),
	}

	return im, queueDir
}

func Test_badCloseThenReopenIndex(t *testing.T) {
	queueDir, err := os.MkdirTemp("", "index_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(queueDir)

	walPath := path.Join(queueDir, "/index_wal")
	indexPath := path.Join(queueDir, "/index")

	im, err := NewIndexManager(walPath, indexPath)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}

	// Add entries to the index
	for i := 0; i < 1000; i++ {
		err := im.Add("example.com", "id"+strconv.Itoa(i), uint64(i*200), uint64(200))
		if err != nil {
			t.Fatalf("failed to add entry to index: %v", err)
		}
	}

	im.Lock()

	// Nil all fiekds to simulate a closed index
	im.hostIndex = nil
	im.walEncoder = nil
	im.walDecoder = nil
	im.indexEncoder = nil
	im.indexDecoder = nil

	// Close file descriptors
	err = im.walFile.Close()
	if err != nil {
		t.Fatalf("failed to close WAL file: %v", err)
	}

	err = im.indexFile.Close()
	if err != nil {
		t.Fatalf("failed to close index file: %v", err)
	}

	im.Unlock()

	im = nil

	im, err = NewIndexManager(walPath, indexPath)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
}

func Test_CloseGracefullyThenReopenIndex(t *testing.T) {
	queueDir, err := os.MkdirTemp("", "index_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(queueDir)

	walPath := path.Join(queueDir, "/index_wal")
	indexPath := path.Join(queueDir, "/index")

	im, err := NewIndexManager(walPath, indexPath)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}

	// Add entries to the index
	for i := 0; i < 1000; i++ {
		err := im.Add("example.com", "id"+strconv.Itoa(i), uint64(i*200), uint64(200))
		if err != nil {
			t.Fatalf("failed to add entry to index: %v", err)
		}
	}

	im.Close()
	im = nil

	im, err = NewIndexManager(walPath, indexPath)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
}

func Benchmark_IndexManager(b *testing.B) {
	benchSizes := []int{100, 1000, 5000}

	fmt.Println(`Running benchmarks for IndexManager...
	- SequentialAddPop: Add and Pop entries sequentially
	- BulkAddThenPop: Add entries in bulk, then Pop them in bulk
Notes:
	- an operation can be either an Add or a Pop
	- ns/op is the average time taken per batch`)

	b.Run("SequentialAddPop", func(b *testing.B) {
		for _, size := range benchSizes {
			b.Run(strconv.Itoa(size), func(b *testing.B) {
				benchmarkSequentialAddPop(b, size)
			})
		}
	})

	b.Run("BulkAddThenPop", func(b *testing.B) {
		for _, size := range benchSizes {
			b.Run(strconv.Itoa(size), func(b *testing.B) {
				benchmarkBulkAddThenPop(b, size)
			})
		}
	})
}

func benchmarkSequentialAddPop(b *testing.B, size int) {
	// Reset the timer to exclude setup time
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		im, tempDir := provideBenchmarkIndexManager(b)
		// Perform size number of Add and Pop operations
		for j := 0; j < size; j++ {
			err := im.Add("example.com", "id", uint64(200), uint64(200))
			if err != nil {
				b.Fatalf("failed to add entry to index: %v", err)
			}

			_, _, _, err = im.Pop("example.com")
			if err != nil {
				b.Fatalf("failed to pop entry from index: %v", err)
			}
		}
		im = nil
		os.RemoveAll(tempDir)
	}

	// Report custom metrics
	b.ReportMetric(float64(b.N), "batches")
	b.ReportMetric(float64(b.N*size*2), "operations")
	b.ReportMetric(float64(b.N*size*2)/b.Elapsed().Seconds(), "ops/s")
}

func benchmarkBulkAddThenPop(b *testing.B, size int) {
	// Reset the timer to exclude setup time
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		im, tempDir := provideBenchmarkIndexManager(b)
		// Add entries
		for j := 0; j < size; j++ {
			err := im.Add("example.com", "id", uint64(200), uint64(200))
			if err != nil {
				b.Fatalf("failed to add entry to index: %v", err)
			}
		}

		// Pop all entries
		for j := 0; j < size; j++ {
			_, _, _, err := im.Pop("example.com")
			if err != nil {
				b.Fatalf("failed to pop entry from index: %v", err)
			}
		}
		im = nil
		os.RemoveAll(tempDir)
	}

	// Report custom metrics
	b.ReportMetric(float64(b.N), "batches")
	b.ReportMetric(float64(b.N*size*2), "operations")
	b.ReportMetric(float64(b.N*size*2)/b.Elapsed().Seconds(), "ops/s")
}
