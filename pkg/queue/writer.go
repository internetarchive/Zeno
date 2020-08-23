package queue

import (
	"github.com/CorentinB/Zeno/pkg/utils"
	"github.com/beeker1121/goque"
	log "github.com/sirupsen/logrus"
)

// NewWriter initialize a receiver channel
func NewWriter() (writerChan chan *Item) {
	return make(chan *Item)
}

// StartWriter listen the channel and write the messages to the queue
func StartWriter(writerChan chan *Item, queue *goque.PriorityQueue) {
	var seencheck []string

	for {
		select {
		case item := <-writerChan:
			if utils.StringInSlice(item.Hash, seencheck) == false {
				_, err := queue.EnqueueObject(item.Hop, item)
				if err != nil {
					log.WithFields(log.Fields{
						"item":  item,
						"error": err,
					}).Info("Unable to enqueue item")
				}
				seencheck = append(seencheck, item.Hash)
			}
		default:
		}
	}
}
