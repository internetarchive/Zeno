package queue

import (
	"encoding/gob"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/queue/index"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
)

type PersistentGroupedQueue struct {
	// Exported fields
	Paused *utils.TAtomBool

	queueFile       *os.File
	metadataFile    *os.File
	metadataEncoder *gob.Encoder
	metadataDecoder *gob.Decoder
	index           *index.IndexManager
	stats           QueueStats
	currentHost     int
	mutex           sync.RWMutex
	statsMutex      sync.RWMutex
	closed          bool

	logger *slog.Logger
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

	indexManager, err := index.NewIndexManager(path.Join(queueDirPath, "index_wal"), path.Join(queueDirPath, "index"))
	if err != nil {
		return nil, fmt.Errorf("create index manager: %w", err)
	}

	q := &PersistentGroupedQueue{
		Paused: new(utils.TAtomBool),

		queueFile:       file,
		metadataFile:    metafile,
		metadataEncoder: gob.NewEncoder(metafile),
		metadataDecoder: gob.NewDecoder(metafile),
		index:           indexManager,
		currentHost:     0,
		stats: QueueStats{
			ElementsPerHost:  make(map[string]int),
			HostDistribution: make(map[string]float64),
		},
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
		return ErrQueueAlreadyClosed
	}

	q.closed = true

	// Save metadata
	err := q.saveMetadata()
	if err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Close the metadata file
	err = q.metadataFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close metadata file: %w", err)
	}

	// Close the main queue file
	err = q.queueFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close queue file: %w", err)
	}

	// Close the index manager
	err = q.index.Close()
	if err != nil {
		return fmt.Errorf("failed to close index manager: %w", err)
	}

	return nil
}
