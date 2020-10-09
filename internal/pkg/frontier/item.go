package frontier

import (
	"net/url"

	"github.com/zeebo/xxh3"
)

// Item is crawl-able object
type Item struct {
	Hash       uint64
	Hop        uint8
	Host       string
	Type       string
	Redirect   int
	URL        *url.URL
	ParentItem *Item
}

// NewItem initialize an *Item
func NewItem(URL *url.URL, parentItem *Item, itemType string, hop uint8) *Item {
	item := new(Item)

	item.URL = URL
	item.Host = URL.Host
	item.Hop = hop
	item.ParentItem = parentItem
	item.Hash = xxh3.HashString(URL.String())
	item.Type = itemType

	return item
}
