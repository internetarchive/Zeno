package queue

import (
	"encoding/gob"
	"errors"
	"fmt"
	"hash/fnv"
	"net/url"
	"os"
	"sync"
	"syscall"

	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/sirupsen/logrus"
)

var (
	ErrQueueFull    = errors.New("queue is full")
	ErrQueueEmpty   = errors.New("queue is empty")
	ErrQueueClosed  = errors.New("queue is closed")
	ErrQueueTimeout = errors.New("queue operation timed out")
)

type LogMessage struct {
	Fields  map[string]interface{}
	Message string
	Level   logrus.Level
}

type PersistentGroupedQueue struct {
	// Exported fields
	Paused      *utils.TAtomBool
	LoggingChan chan *LogMessage

	file        *os.File
	data        []byte
	size        uint64
	metadata    *os.File
	hostIndex   map[string][]uint64
	hostOrder   []string
	currentHost int
	mutex       sync.RWMutex
	hostMutex   sync.Mutex
	statsMutex  sync.RWMutex
	enqueueChan chan *Item
	dequeueChan chan chan *Item
	stats       QueueStats
	encoder     *gob.Encoder
	decoder     *gob.Decoder
	wg          sync.WaitGroup
	done        chan struct{}
	closed      bool
}

type Item struct {
	ID              string
	Hash            uint64
	Hop             uint8
	Host            string
	Type            string
	Redirect        int
	URL             *url.URL
	ParentItem      *Item
	LocallyCrawled  uint64
	BypassSeencheck string
}

func NewPersistentGroupedQueue(filename string, loggingChan chan *LogMessage, size uint64) (*PersistentGroupedQueue, error) {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("open queue file: %w", err)
	}
	defer func() {
		if err != nil {
			file.Close()
		}
	}()

	if err = file.Truncate(int64(size)); err != nil {
		return nil, fmt.Errorf("set queue file size: %w", err)
	}

	data, err := syscall.Mmap(int(file.Fd()), 0, int(size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("memory-map queue file: %w", err)
	}

	metafile, err := os.OpenFile(filename+".meta", os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		syscall.Munmap(data)
		return nil, fmt.Errorf("open metadata file: %w", err)
	}

	q := &PersistentGroupedQueue{
		Paused:      new(utils.TAtomBool),
		LoggingChan: loggingChan,

		file:        file,
		data:        data,
		size:        size,
		metadata:    metafile,
		hostIndex:   make(map[string][]uint64),
		hostOrder:   []string{},
		currentHost: 0,
		enqueueChan: make(chan *Item, 1000),
		dequeueChan: make(chan chan *Item, 1000),
		stats: QueueStats{
			ElementsPerHost:  make(map[string]int),
			HostDistribution: make(map[string]float64),
			TotalSize:        size,
		},
		encoder: gob.NewEncoder(metafile),
		decoder: gob.NewDecoder(metafile),
		done:    make(chan struct{}),
	}

	if err = q.loadMetadata(); err != nil {
		q.Close()
		return nil, fmt.Errorf("load queue metadata: %w", err)
	}

	q.wg.Add(2)
	go q.enqueueWorker()
	go q.dequeueWorker()

	return q, nil
}

func (q *PersistentGroupedQueue) Close() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.closed {
		return nil // Already closed
	}

	// Signal the worker goroutines to stop
	close(q.done)

	// Wait for worker goroutines to finish
	q.wg.Wait()

	// Now it's safe to close the channels
	close(q.enqueueChan)
	close(q.dequeueChan)

	q.closed = true

	// Save metadata
	err := q.saveMetadata()
	if err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Unmap the memory-mapped file
	err = syscall.Munmap(q.data)
	if err != nil {
		return fmt.Errorf("failed to unmap queue data: %w", err)
	}

	// Close the main queue file
	err = q.file.Close()
	if err != nil {
		return fmt.Errorf("failed to close queue file: %w", err)
	}

	// Close the metadata file
	err = q.metadata.Close()
	if err != nil {
		return fmt.Errorf("failed to close metadata file: %w", err)
	}

	return nil
}

// NewItem initialize an *Item
func NewItem(URL *url.URL, parentItem *Item, itemType string, hop uint8, ID string, bypassSeencheck bool) *Item {
	item := new(Item)

	item.URL = URL
	if ID != "" {
		item.ID = ID
	}
	item.Host = URL.Host
	item.Hop = hop
	item.ParentItem = parentItem

	h := fnv.New64a()
	h.Write([]byte(utils.URLToString(URL)))
	item.Hash = h.Sum64()

	item.Type = itemType

	// The reason we are using a string instead of a bool is because
	// gob's encode/decode doesn't properly support booleans
	if bypassSeencheck {
		item.BypassSeencheck = "true"
	} else {
		item.BypassSeencheck = "false"
	}

	return item
}
