package frontier

import (
	"net/url"

	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/zeebo/xxh3"
)

// Item is crawl-able object
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
	BypassSeencheck bool
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
	item.Hash = xxh3.HashString(utils.URLToString(URL))
	item.Type = itemType
	item.BypassSeencheck = bypassSeencheck

	return item
}
