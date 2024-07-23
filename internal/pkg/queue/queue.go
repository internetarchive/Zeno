package queue

import (
	"encoding/gob"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/utils"
)

var (
	ErrQueueFull    = errors.New("queue is full")
	ErrQueueEmpty   = errors.New("queue is empty")
	ErrQueueClosed  = errors.New("queue is closed")
	ErrQueueTimeout = errors.New("queue operation timed out")
)

type PersistentGroupedQueue struct {
	// Exported fields
	Paused *utils.TAtomBool

	queueDirPath    string
	queueFile       *os.File
	metadataFile    *os.File
	metadataEncoder *gob.Encoder
	metadataDecoder *gob.Decoder
	hostIndex       map[string][]uint64
	stats           QueueStats
	hostOrder       []string
	currentHost     int
	mutex           sync.RWMutex
	statsMutex      sync.RWMutex
	hostMutex       sync.Mutex
	closed          bool
}

type Item struct {
	URL             *url.URL
	ParentURL       *url.URL
	Hop             uint64
	Type            string
	ID              string
	BypassSeencheck bool
	Hash            uint64
	LocallyCrawled  uint64
	Redirect        uint64
}

func NewPersistentGroupedQueue(queueDirPath string) (*PersistentGroupedQueue, error) {
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
		Paused: new(utils.TAtomBool),

		queueDirPath:    queueDirPath,
		queueFile:       file,
		metadataFile:    metafile,
		metadataEncoder: gob.NewEncoder(metafile),
		metadataDecoder: gob.NewDecoder(metafile),
		hostIndex:       make(map[string][]uint64),
		hostOrder:       []string{},
		currentHost:     0,
		stats: QueueStats{
			ElementsPerHost:  make(map[string]int),
			HostDistribution: make(map[string]float64),
		},
	}

	// Loading stats from the disk means deleting the file from disk after having read it
	if err = q.loadStatsFromFile(path.Join(q.queueDirPath, "queue.stats")); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("load queue stats: %w", err)
		}
	}

	if err = q.loadMetadata(); err != nil {
		q.Close()
		return nil, fmt.Errorf("load queue metadata: %w", err)
	}

	return q, nil
}

func (q *PersistentGroupedQueue) Close() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.closed {
		return nil // Already closed
	}

	q.closed = true

	// Save metadata
	err := q.saveStatsToFile(path.Join(q.queueDirPath, "queue.stats"))
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
