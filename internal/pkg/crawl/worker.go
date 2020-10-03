package crawl

import (
	"github.com/CorentinB/Zeno/internal/pkg/frontier"

	log "github.com/sirupsen/logrus"
)

// Worker archive the items!
func (c *Crawl) Worker(item *frontier.Item) {
	// Capture the page
	outlinks, err := c.capture(item)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Debug(item.URL.String())
		return
	}

	// Send the outlinks to the pool of workers
	if item.Hop < c.MaxHops {
		for _, outlink := range outlinks {
			outlink := outlink
			newItem := frontier.NewItem(&outlink, item, item.Hop+1)
			c.Frontier.PushChan <- newItem
		}
	}
}
