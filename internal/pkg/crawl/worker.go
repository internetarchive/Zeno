package crawl

import (
	"context"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/chromedp/chromedp"

	log "github.com/sirupsen/logrus"
)

// Worker archive the items!
func (c *Crawl) Worker(item *frontier.Item) {
	// Create context for headless browser
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	c.ActiveWorkers.Incr(1)

	// Capture the page
	outlinks, err := c.capture(ctx, item)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Debug(item.URL.String())
		c.ActiveWorkers.Incr(-1)
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

	c.ActiveWorkers.Incr(-1)
}
