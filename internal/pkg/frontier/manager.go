package frontier

import (
	"strconv"

	"github.com/paulbellamy/ratecounter"
	"github.com/sirupsen/logrus"
)

func (f *Frontier) writeItemsToQueue() {
	for item := range f.PushChan {
		// If --seencheck is enabled, then we check if the URI is in the
		// seencheck DB before doing anything. If it is in it, we skip the item
		if f.UseSeencheck {
			hash := strconv.FormatUint(item.Hash, 10)
			if f.Seencheck.IsSeen(hash) {
				continue
			} else {
				f.Seencheck.Seen(hash)
			}
		}

		// Increment the counter of the host in the hosts pool,
		// if the hosts doesn't exist in the pool, it will be created
		f.HostPool.Incr(item.Host)

		// Add the item to the host's queue
		_, err := f.Queue.EnqueueObject([]byte(item.Host), item)
		if err != nil {
			log.WithFields(logrus.Fields{
				"error": err,
				"item":  item,
			}).Error("Unable to enqueue item")
		}
		f.QueueCount.Incr(1)

		log.WithFields(logrus.Fields{
			"url": item.URL,
		}).Debug("Item enqueued")
	}
}

func (f *Frontier) readItemsFromQueue() {
	var mapCopy map[string]*ratecounter.Counter

	for {
		// We cleanup the hosts pool by removing
		// all the hosts with a count of 0, then
		// we make a snapshot of the hosts
		// pool that we will iterate on
		f.HostPool.DeleteEmptyHosts()
		mapCopy = make(map[string]*ratecounter.Counter, 0)
		f.HostPool.Lock()
		for key, val := range f.HostPool.Hosts {
			mapCopy[key] = val
		}
		f.HostPool.Unlock()

		// We iterate over the copied pool, and dequeue
		// new URLs to crawl based on that hosts pool
		// that allow us to crawl a wide variety of domains
		// at the same time, maximizing our speed
		for host := range mapCopy {
			if f.HostPool.GetCount(host) == 0 {
				continue
			}

			// Dequeue an item from the local queue
			queueItem, err := f.Queue.DequeueString(host)
			if err != nil {
				log.WithFields(logrus.Fields{
					"error": err,
				}).Debug("Unable to dequeue item")
				continue
			}
			f.QueueCount.Incr(-1)

			// Turn the item from the queue into an Item
			var item *Item
			err = queueItem.ToObject(&item)
			if err != nil {
				log.WithFields(logrus.Fields{
					"error": err,
				}).Error("Unable to parse queue's item")
				continue
			}

			// Sending the item to the workers via PullChan
			f.PullChan <- item
			log.WithFields(logrus.Fields{
				"url": item.URL,
			}).Debug("Item sent to workers pool")

			f.HostPool.Decr(host)
		}
	}
}
