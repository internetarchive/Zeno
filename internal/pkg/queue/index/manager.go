package index

import (
	"encoding/gob"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/log"
)

var dumpFrequency = 60 // seconds

type Operation int

const (
	OpAdd Operation = iota
	OpPop
)

type WALEntry struct {
	Op       Operation
	Host     string
	BlobID   string
	Position uint64
	Size     uint64
}

type IndexManager struct {
	sync.Mutex
	hostIndex    *Index
	walFile      *os.File
	indexFile    *os.File
	walEncoder   *gob.Encoder
	indexEncoder *gob.Encoder
	indexDecoder *gob.Decoder
	dumpTicker   *time.Ticker
	lastDumpTime time.Time
	opsSinceDump int
	logger       *log.Entry
	totalOps     uint64
}

func NewIndexManager(walPath, indexPath string, logger *log.Entry) (*IndexManager, error) {
	if logger == nil {
		fmt.Printf("logger is nil")
	}

	walFile, err := os.OpenFile(walPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	indexFile, err := os.OpenFile(indexPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		walFile.Close()
		return nil, fmt.Errorf("failed to open index file: %w", err)
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

	err = im.loadIndex()
	if err != nil {
		walFile.Close()
		indexFile.Close()
		return nil, fmt.Errorf("failed to load index: %w", err)
	}

	go im.periodicDump()

	return im, nil
}

func (im *IndexManager) Add(host string, id string, position uint64, size uint64) error {
	im.Lock()
	defer im.Unlock()

	// Write to WAL
	err := im.writeToWAL(OpAdd, host, id, position, size)
	if err != nil {
		return fmt.Errorf("failed to write to WAL: %w", err)
	}

	// Update in-memory index
	if err := im.hostIndex.add(host, id, position, size); err != nil {
		return fmt.Errorf("failed to update in-memory index: %w", err)
	}

	im.opsSinceDump++
	im.totalOps++

	return nil
}

func (im *IndexManager) Pop(host string) (id string, position uint64, size uint64, err error) {
	im.Lock()
	defer im.Unlock()

	// Prepare the channels
	getChan := make(chan *blob)
	WALChan := make(chan bool)

	go func() {
		// Write to WAL
		blob := <-getChan
		err := im.writeToWAL(OpPop, host, blob.id, blob.position, blob.size)
		if err != nil {
			im.logger.Error("failed to write to WAL", "error", err)
			panic(err)
		}
		id = blob.id
		position = blob.position
		size = blob.size
		WALChan <- true
	}()

	// Pop from in-memory index
	err = im.hostIndex.pop(host, getChan, WALChan)
	if err != nil {
		return "", 0, 0, err
	}

	im.opsSinceDump++
	im.totalOps++

	return
}

func (im *IndexManager) Close() error {
	im.dumpTicker.Stop()
	if err := im.performDump(); err != nil {
		return fmt.Errorf("failed to perform final dump: %w", err)
	}
	if err := im.walFile.Close(); err != nil {
		return fmt.Errorf("failed to close WAL file: %w", err)
	}
	if err := im.indexFile.Close(); err != nil {
		return fmt.Errorf("failed to close index file: %w", err)
	}
	return nil
}

func (im *IndexManager) GetStats() string {
	im.Lock()
	defer im.Unlock()

	return fmt.Sprintf("Total operations: %d, Operations since last dump: %d",
		im.totalOps, im.opsSinceDump)
}

// GetHosts returns a list of all hosts in the index
func (im *IndexManager) GetHosts() []string {
	im.Lock()
	defer im.Unlock()

	return im.hostIndex.getOrderedHosts()
}

func (im *IndexManager) IsEmpty() bool {
	im.Lock()
	defer im.Unlock()

	im.hostIndex.Lock()
	defer im.hostIndex.Unlock()

	if len(im.hostIndex.index) == 0 {
		return true
	}
	return false
}
