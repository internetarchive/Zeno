package queue

import "net/url"

// Item is crawl-able object
type Item struct {
	Hop        uint8
	Host       string
	URL        *url.URL
	ParentItem *Item
}

// NewItem initialize an *Item
func NewItem(URL *url.URL, parentItem *Item, hop uint8) *Item {
	item := new(Item)

	item.Hop = hop
	item.Host = URL.Host
	item.URL = URL
	item.ParentItem = parentItem

	return item
}
