package crawl

import (
	"strings"
	"time"

	"github.com/CorentinB/Zeno/pkg/queue"
	log "github.com/sirupsen/logrus"
)

// Worker archive the items!
func (c *Crawl) Worker(writerChan chan *queue.Item) {
	// Get item from queue
	for {
		// Dequeue an item from the local queue
		queueItem, err := c.Queue.Dequeue()
		if err != nil {
			log.WithFields(log.Fields{
				"item":  queueItem,
				"error": err,
			}).Error("Unable to dequeue item")

			// If the queue is empty, we wait 5 seconds
			if strings.Compare(err.Error(), "goque: Stack or queue is empty") == 0 {
				time.Sleep(1 * time.Second)
			}

			continue
		}

		// Turn the item from the queue into an Item
		var item *queue.Item
		err = queueItem.ToObject(&item)
		if err != nil {
			log.WithFields(log.Fields{
				"item":  queueItem,
				"error": err,
			}).Error("Unable to parse queue's item")
		}

		// Capture the page
		outlinks, err := c.Capture(item)
		if err != nil {
			log.WithFields(log.Fields{
				"item":  queueItem,
				"error": err,
			}).Error("Error when capturing URL")
			continue
		}

		// Send the outlinks for queuing
		if item.Hop < c.MaxHops {
			for _, outlink := range outlinks {
				newItem := queue.NewItem(&outlink, item, item.Hop+1)
				writerChan <- newItem
			}
		}
	}
}
