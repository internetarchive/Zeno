package queue

import (
	"encoding/gob"
	"errors"
	"fmt"
	"hash/fnv"
	"net/url"
	"os"
	"path"
	"sync"

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

	queueEncoder    *gob.Encoder
	queueDecoder    *gob.Decoder
	queueFile       *os.File
	metadataFile    *os.File
	metadataEncoder *gob.Encoder
	metadataDecoder *gob.Decoder
	hostIndex       map[string][]uint64
	hostOrder       []string
	currentHost     int
	mutex           sync.RWMutex
	hostMutex       sync.Mutex
	statsMutex      sync.RWMutex
	enqueueChan     chan *Item
	dequeueChan     chan chan *Item
	stats           QueueStats
	wg              sync.WaitGroup
	done            chan struct{}
	closed          bool
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

func NewPersistentGroupedQueue(queueDirPath string, loggingChan chan *LogMessage) (*PersistentGroupedQueue, error) {
	err := os.MkdirAll(queueDirPath, 0755)
	if err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path.Join(queueDirPath, "queue"), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("open queue file: %w", err)
	}

	metafile, err := os.OpenFile(path.Join(queueDirPath, "queue.meta"), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("open metadata file: %w", err)
	}

	q := &PersistentGroupedQueue{
		Paused:      new(utils.TAtomBool),
		LoggingChan: loggingChan,

		queueFile:       file,
		queueEncoder:    gob.NewEncoder(file),
		queueDecoder:    gob.NewDecoder(file),
		metadataFile:    metafile,
		metadataEncoder: gob.NewEncoder(metafile),
		metadataDecoder: gob.NewDecoder(metafile),
		hostIndex:       make(map[string][]uint64),
		hostOrder:       []string{},
		currentHost:     0,
		enqueueChan:     make(chan *Item, 1000),
		dequeueChan:     make(chan chan *Item, 1000),
		stats: QueueStats{
			ElementsPerHost:  make(map[string]int),
			HostDistribution: make(map[string]float64),
		},
		done: make(chan struct{}),
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

	// Close the main queue file
	err = q.queueFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close queue file: %w", err)
	}

	// Close the metadata file
	err = q.metadataFile.Close()
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
