package crawl

import (
	"github.com/CorentinB/Zeno/pkg/queue"
	log "github.com/sirupsen/logrus"
)

func (c *Crawl) Manager(pullChan, pushChan chan *queue.Item) {
	for {
		select {
		case item := <-pullChan:
			select {
			case pushChan <- item:
				// URL is sent to a worker
				log.WithFields(log.Fields{
					"url": item.URL,
				}).Debug("URL sent to workers pool")
			default:
				// No worker available to receive the seed, we pile it up in the queue
				_, err := c.Queue.EnqueueObject(item.Hop, item)
				if err != nil {
					log.WithFields(log.Fields{
						"error": err,
					}).Error("Unable to enqueue item")
				}
				log.WithFields(log.Fields{
					"url": item.URL,
				}).Debug("URL added to queue for processing")
			}
		default:
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

			select {
			case pushChan <- item:
				// URL is sent to a worker
				log.WithFields(log.Fields{
					"url": item.URL,
				}).Debug("URL sent to workers pool")
			default:
				// Item can't be send, it's added back in the queue
				_, err := c.Queue.EnqueueObject(item.Hop, item)
				if err != nil {
					log.WithFields(log.Fields{
						"error": err,
					}).Error("Unable to enqueue item")
				}
			}
		}
	}
}
