package crawl

import (
	"github.com/CorentinB/Zeno/pkg/queue"
	log "github.com/sirupsen/logrus"
)

func (c *Crawl) writeItemsToQueue(pullChan <-chan *queue.Item) {
	for item := range pullChan {
		_, err := c.Queue.EnqueueObject(item.Hop, item)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Unable to enqueue item")
		}
		log.WithFields(log.Fields{
			"url": item.URL,
		}).Debug("Item enqueued")
	}
}

func (c *Crawl) readItemsFromQueue(outChan chan *queue.Item) {
	for {
		// Dequeue an item from the local queue
		queueItem, err := c.Queue.Dequeue()
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Debug("Unable to dequeue item")
			continue
		}

		// Turn the item from the queue into an Item
		var item *queue.Item
		err = queueItem.ToObject(&item)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Unable to parse queue's item")
			continue
		}

		// Sending the item to the workers via outChan
		outChan <- item
		log.WithFields(log.Fields{
			"url": item.URL,
		}).Debug("Item sent to workers pool")
	}
}

func (c *Crawl) Manager(inChan, outChan chan *queue.Item) {
	// Function responsible for writing the items received on inChan to the
	// local queue, items received on this channels are typically initial seeds
	// or outlinks discovered on web pages
	go c.writeItemsToQueue(inChan)

	// Function responsible fro reading the items from the queue and dispatching
	// them to the workers listening on the outChan
	go c.readItemsFromQueue(outChan)
}
