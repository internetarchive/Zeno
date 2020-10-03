package frontier

import (
	"net/url"
	"path"

	"github.com/beeker1121/goque"
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

func newPersistentQueue(jobPath string) (queue *goque.PrefixQueue, err error) {
	// Initialize a prefix queue
	queue, err = goque.OpenPrefixQueue(path.Join(jobPath, "queue"))
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Unable to create prefix queue")
		return nil, err
	}

	return queue, nil
}
