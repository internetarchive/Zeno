package crawl

import (
	"context"

	"github.com/chromedp/chromedp"
	swg "github.com/remeh/sizedwaitgroup"

	"github.com/CorentinB/Zeno/pkg/queue"
	log "github.com/sirupsen/logrus"
)

// Worker archive the items!
func (c *Crawl) Worker(pullChan, pushChan chan *queue.Item, worker *swg.SizedWaitGroup) {
	defer worker.Done()

	// Create context for headless browser
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	for item := range pullChan {
		c.ActiveWorkers.Incr(1)

		// Capture the page
		outlinks, err := c.Capture(ctx, item)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error(item.URL.String())
			c.ActiveWorkers.Incr(-1)
			continue
		}

		// Send the outlinks to the pool of workers
		if item.Hop < c.MaxHops {
			for _, outlink := range outlinks {
				outlink := outlink
				newItem := queue.NewItem(&outlink, item, item.Hop+1)
				pushChan <- newItem
			}
		}

		c.ActiveWorkers.Incr(-1)
	}
}
