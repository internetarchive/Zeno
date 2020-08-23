package queue

import (
	"github.com/CorentinB/Zeno/pkg/utils"
	"net/url"
)

// Item is crawl-able object
type Item struct {
	Hash string
	Hop        uint8
	Host       string
	URL        *url.URL
	ParentItem *Item
}

// NewItem initialize an *Item
func NewItem(URL *url.URL, parentItem *Item, hop uint8) *Item {
	item := new(Item)

	item.Hash = utils.GetSHA1(URL.String())
	item.Hop = hop
	item.Host = URL.Host
	item.URL = URL
	item.ParentItem = parentItem

	return item
}
