package crawl

import (
	"github.com/CorentinB/Zeno/pkg/queue"
	log "github.com/sirupsen/logrus"
)

func (c *Crawl) writeItemsToQueue(pullChan chan *queue.Item) {
	for item := range pullChan {
		// Check if host is in the pool, if it is not, we add it
		// if it is, we increment its counter
		c.HostPool.Mutex.Lock()
		if c.HostPool.IsHostInPool(item.Host) == false {
			c.HostPool.Add(item.Host)
		} else {
			c.HostPool.Incr(item.Host)
		}
		c.HostPool.Mutex.Unlock()

		// Add the item to the host's queue
		_, err := c.Queue.EnqueueObject([]byte(item.Host), item)
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
		c.HostPool.Mutex.Lock()
		for _, host := range c.HostPool.Hosts {
			if host.Count.Value() == 0 {
				continue
			}

			// Dequeue an item from the local queue
			queueItem, err := c.Queue.Dequeue([]byte(host.Host))
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

			host.Count.Incr(-1)
		}
		c.HostPool.Mutex.Unlock()
	}
}

// Manager manage the crawl frontier
func (c *Crawl) Manager(inChan, outChan chan *queue.Item) {
	// Function responsible for writing the items received on inChan to the
	// local queue, items received on this channels are typically initial seeds
	// or outlinks discovered on web pages
	go c.writeItemsToQueue(inChan)

	// Function responsible fro reading the items from the queue and dispatching
	// them to the workers listening on the outChan
	go c.readItemsFromQueue(outChan)
}
