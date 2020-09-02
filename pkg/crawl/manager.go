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
		c.Queue.PushBack(item)
		mutex.Unlock()

		log.WithFields(log.Fields{
			"url": item.URL,
		}).Debug("Item enqueued")
	}
}

func (c *Crawl) readItemsFromQueue(outChan chan *queue.Item, mutex *sync.Mutex) {
	for {
		mutex.Lock()
		if c.Queue.Len() > 0 {
			// Dequeue an item from the queue
			itemQueue := c.Queue.Front()
			item, ok := itemQueue.Value.(*queue.Item)
			if ok {
				// Sending the item to the workers via outChan
				outChan <- item
				log.WithFields(log.Fields{
					"url": item.URL,
				}).Debug("Item sent to workers pool")
			}
			c.Queue.Remove(itemQueue)
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
