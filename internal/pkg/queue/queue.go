package queue

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"net/url"
	"os"
	"path"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
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
	cond            *sync.Cond
	stats           QueueStats
	hostOrder       []string
	currentHost     int
	mutex           sync.RWMutex
	statsMutex      sync.RWMutex
	hostMutex       sync.Mutex
	closed          bool
}

type Item struct {
	*ProtoItem
	URL    *url.URL
	Parent *Item
}

func init() {
	gob.Register(Item{})
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
		stats: QueueStats{
			ElementsPerHost:  make(map[string]int),
			HostDistribution: make(map[string]float64),
		},
	}

	q.cond = sync.NewCond(&q.mutex)

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

func NewItem(URL *url.URL, parentItem *Item, itemType string, hop uint64, ID string, bypassSeencheck bool) (*Item, error) {
	urlJSON, err := json.Marshal(URL)
	if err != nil {
		return nil, err
	}

	var parentItemBytes []byte
	if parentItem != nil {
		parentItemBytes, _ = proto.Marshal(parentItem.ProtoItem)
	}

	protoItem := &ProtoItem{
		Url:             urlJSON,
		ID:              ID,
		Host:            URL.Host,
		Hop:             hop,
		Type:            itemType,
		BypassSeencheck: bypassSeencheck,
		ParentItem:      parentItemBytes,
	}

	h := fnv.New64a()
	h.Write([]byte(URL.String()))
	protoItem.Hash = h.Sum64()

	return &Item{
		ProtoItem: protoItem,
		URL:       URL,
		Parent:    parentItem,
	}, nil
}

func (i *Item) UnmarshalParent() error {
	if i.ParentItem == nil || len(i.ParentItem) == 0 {
		return nil
	}

	parentProtoItem := &ProtoItem{}
	err := proto.Unmarshal(i.ParentItem, parentProtoItem)
	if err != nil {
		return err
	}

	var parentURL url.URL
	err = json.Unmarshal(parentProtoItem.Url, &parentURL)
	if err != nil {
		return err
	}

	i.Parent = &Item{
		ProtoItem: parentProtoItem,
		URL:       &parentURL,
	}

	return i.Parent.UnmarshalParent()
}
