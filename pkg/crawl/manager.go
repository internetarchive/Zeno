package crawl

import (
	"sync"

	"github.com/CorentinB/Zeno/pkg/queue"
	log "github.com/sirupsen/logrus"
)

func (c *Crawl) writeItemsToQueue(pullChan <-chan *queue.Item, mutex *sync.Mutex) {
	for item := range pullChan {
		// Add item to queue
		mutex.Lock()
		c.Queue = append(c.Queue, *item)
		mutex.Unlock()

		log.WithFields(log.Fields{
			"url": item.URL,
		}).Debug("Item enqueued")
	}
}

func (c *Crawl) readItemsFromQueue(outChan chan *queue.Item, mutex *sync.Mutex) {
	var item queue.Item

	for {
		mutex.Lock()
		if len(c.Queue) > 0 {
			// Dequeue an item from the queue
			item, c.Queue = c.Queue[0], c.Queue[1:]

			// Sending the item to the workers via outChan
			item := item
			outChan <- &item
			log.WithFields(log.Fields{
				"url": item.URL,
			}).Debug("Item sent to workers pool")
		}
		mutex.Unlock()
	}
}

// Manager manage the crawl frontier
func (c *Crawl) Manager(inChan, outChan chan *queue.Item) {
	var mutex sync.Mutex

	// Function responsible for writing the items received on inChan to the
	// local queue, items received on this channels are typically initial seeds
	// or outlinks discovered on web pages
	go c.writeItemsToQueue(inChan, &mutex)

	// Function responsible fro reading the items from the queue and dispatching
	// them to the workers listening on the outChan
	go c.readItemsFromQueue(outChan, &mutex)
}
