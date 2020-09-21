package frontier

import (
	"net/url"
	"path/filepath"

	"github.com/beeker1121/goque"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/zeebo/xxh3"
)

// Item is crawl-able object
type Item struct {
	Hash       uint64
	Hop        uint8
	Host       string
	URL        *url.URL
	ParentItem *Item
}

// NewItem initialize an *Item
func NewItem(URL *url.URL, parentItem *Item, hop uint8) *Item {
	item := new(Item)

	item.URL = URL
	item.Host = URL.Host
	item.Hop = hop
	item.ParentItem = parentItem
	item.Hash = xxh3.HashString(URL.String())

	return item
}

func newPersistentQueue() (queue *goque.PrefixQueue, err error) {
	// All on-disk queues are in the "./jobs" directory
	queueUUID, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}
	queuePath := filepath.Join(".", "jobs", queueUUID.String())

	// Initialize a prefix queue
	queue, err = goque.OpenPrefixQueue(queuePath)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Unable to create prefix queue")
		return nil, err
	}

	return queue, nil
}
