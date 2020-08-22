package crawl

import (
	"strings"
	"time"

	"github.com/CorentinB/Zeno/pkg/queue"
	"github.com/beeker1121/goque"
	log "github.com/sirupsen/logrus"
)

// Worker archive the items!
func Worker(writerChan chan *queue.Item, localQueue *goque.PriorityQueue) {
	// Get item from queue
	for {
		// Dequeue an item from the local queue
		queueItem, err := localQueue.Dequeue()
		if err != nil {
			log.WithFields(log.Fields{
				"item":  queueItem,
				"error": err,
			}).Error("Unable to dequeue item")

			// If the queue is empty, we wait 5 seconds
			if strings.Compare(err.Error(), "goque: Stack or queue is empty") == 0 {
				time.Sleep(3 * time.Second)
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
	}
}
