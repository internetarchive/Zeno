package frontier

import (
	"net/url"
	"path/filepath"

	"github.com/beeker1121/goque"
	"github.com/google/uuid"
)

// Item is crawl-able object
type Item struct {
	Hash       string
	Hop        uint8
	Host       string
	URL        *url.URL
	ParentItem *Item
}

// NewItem initialize an *Item
func NewItem(URL *url.URL, parentItem *Item, hop uint8) *Item {
	item := new(Item)

	//item.Hash = utils.GetSHA1(URL.String())
	item.Hop = hop
	item.Host = URL.Host
	item.URL = URL
	item.ParentItem = parentItem

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
		return nil, err
	}

	return queue, nil
}
