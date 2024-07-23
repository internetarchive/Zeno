package index

import (
	"encoding/gob"
	"os"
	"path"
	"testing"
	"time"
)

func Test_isWALEmpty(t *testing.T) {
	queueDir, err := os.MkdirTemp("", "index_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(queueDir)

	walPath := path.Join(queueDir, "/index_wal")
	indexPath := path.Join(queueDir, "/index")

	walFile, err := os.OpenFile(walPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open WAL file: %v", err)
	}

	indexFile, err := os.OpenFile(indexPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		walFile.Close()
		t.Fatalf("failed to open index file: %v", err)
	}

	im := &IndexManager{
		hostIndex:    nil,
		walFile:      walFile,
		indexFile:    indexFile,
		walEncoder:   gob.NewEncoder(walFile),
		indexEncoder: gob.NewEncoder(indexFile),
		indexDecoder: gob.NewDecoder(indexFile),
		dumpTicker:   time.NewTicker(time.Duration(dumpFrequency) * time.Second),
		lastDumpTime: time.Now(),
	}

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
