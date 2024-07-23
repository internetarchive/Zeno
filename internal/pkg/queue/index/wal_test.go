package index

import (
	"encoding/gob"
	"os"
	"path"
	"testing"
	"time"
)

func provideTestIndexManager(t *testing.T) *IndexManager {
	queueDir, err := os.MkdirTemp("", "index_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(queueDir)

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

	return im
}

func Test_isWALEmpty(t *testing.T) {
	im := provideTestIndexManager(t)

	isEmpty, err := im.isWALEmpty()
	if err != nil {
		t.Fatalf("Failed to check if WAL is empty: %v", err)
	}
	if !isEmpty {
		t.Error("Expected WAL to be empty")
	}

	// Write to WAL
	err = im.writeToWAL(OpAdd, "example.com", "id", 0, 0)
	if err != nil {
		t.Fatalf("Failed to write to WAL: %v", err)
	}

	isEmpty, err = im.isWALEmpty()
	if err != nil {
		t.Fatalf("Failed to check if WAL is empty: %v", err)
	}
	if isEmpty {
		t.Error("Expected WAL not to be empty")
	}
}

func Test_replayWAL(t *testing.T) {
	im := provideTestIndexManager(t)

	var replayedEntries int

	// Write to WAL
	err := im.writeToWAL(OpAdd, "example.com", "id", 0, 200)
	if err != nil {
		t.Fatalf("Failed to write to WAL: %v", err)
	}

	// Replay WAL
	err = im.replayWAL(&replayedEntries)
	if err != nil {
		t.Fatalf("Failed to replay WAL: %v", err)
	}

	if replayedEntries != 1 {
		t.Errorf("Expected 1 entry to be replayed, got: %d", replayedEntries)
	}
}

func Test_bigreplayWAL(t *testing.T) {
	im := provideTestIndexManager(t)

	numEntries := 1000

	// Write to WAL
	for i := 0; i < numEntries; i++ {
		err := im.writeToWAL(OpAdd, "example.com", "id", 0, 200)
		if err != nil {
			t.Fatalf("Failed to write to WAL: %v", err)
		}
	}

	// Replay WAL
	var replayedEntries int
	err := im.replayWAL(&replayedEntries)
	if err != nil {
		t.Fatalf("Failed to replay WAL: %v", err)
	}

	if replayedEntries != numEntries {
		t.Errorf("Expected %d entries to be replayed, got: %d", numEntries, replayedEntries)
	}
}
