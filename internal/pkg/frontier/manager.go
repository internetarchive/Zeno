package frontier

import (
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

func (f *Frontier) writeItemsToQueue() {
	f.IsQueueWriterActive.Set(true)

	for item := range f.PushChan {
		item := item

		if f.Paused.Get() {
			time.Sleep(time.Second)
		}

		// If --seencheck is enabled, then we check if the URI is in the
		// seencheck DB before doing anything. If it is in it, we skip the item
		if f.UseSeencheck {
			hash := strconv.FormatUint(item.Hash, 10)

			found, value := f.Seencheck.IsSeen(hash)
			if !found || (value == "asset" && item.Type == "seed") {
				f.Seencheck.Seen(hash, item.Type)
			} else {
				continue
			}
		}

		// Increment the counter of the host in the hosts pool,
		// if the hosts doesn't exist in the pool, it will be created
		f.IncrHost(item.Host)

		// Add the item to the host's queue
		_, err := f.Queue.EnqueueObject([]byte(item.Host), item)
		if err != nil {
			f.LoggingChan <- &FrontierLogMessage{
				Fields: logrus.Fields{
					"err":  err.Error(),
					"item": item,
				},
				Message: "unable to enqueue item",
				Level:   logrus.ErrorLevel,
			}
		}

		f.QueueCount.Incr(1)

		// loggingChan <- &FrontierLogMessage{
		// 	Fields: logrus.Fields{
		// 		"url": item.URL,
		// 	},
		// 	Message: "item enqueued",
		// 	Level:   logrus.DebugLevel,
		// }
	}

	if f.FinishingQueueWriter.Get() {
		f.IsQueueWriterActive.Set(false)
		return
	}
}

func (f *Frontier) readItemsFromQueue() {
	f.IsQueueReaderActive.Set(true)

	if f.QueueCount.Value() == 0 {
		time.Sleep(time.Second)
	}

	for {
		if f.FinishingQueueReader.Get() {
			f.IsQueueReaderActive.Set(false)
			return
		}

		if f.Paused.Get() {
			time.Sleep(time.Second)
		}

		// We iterate over the copied pool, and dequeue
		// new URLs to crawl based on that hosts pool
		// that allow us to crawl a wide variety of domains
		// at the same time, maximizing our speed
		f.HostPool.Range(func(host any, count any) bool {
			if f.Paused.Get() {
				time.Sleep(time.Second)
			}

			//logrus.Infof("host: %s, active: %d, total: %d", host.(string), f.GetActiveHostCount(host.(string)), f.GetHostCount(host.(string)))

			if f.GetHostCount(host.(string)) == 0 {
				return true
			}

			// Dequeue an item from the local queue
			queueItem, err := f.Queue.DequeueString(host.(string))
			if err != nil {
				f.LoggingChan <- &FrontierLogMessage{
					Fields: logrus.Fields{
						"err":  err.Error(),
						"host": host,
					},
					Message: "unable to dequeue item",
					Level:   logrus.DebugLevel,
				}

				if err.Error() == "goque: ID used is outside range of stack or queue" {
					f.DecrHost(host.(string))
				}

				return true
			}

			f.QueueCount.Incr(-1)

			// Turn the item from the queue into an Item
			var item *Item
			err = queueItem.ToObject(&item)
			if err != nil {
				f.LoggingChan <- &FrontierLogMessage{
					Fields: logrus.Fields{
						"err":  err.Error(),
						"item": queueItem,
					},
					Message: "unable to parse queue's item",
					Level:   logrus.ErrorLevel,
				}

				return true
			}

			// Sending the item to the workers via PullChan
			f.PullChan <- item

			f.DecrHost(host.(string))

			if f.FinishingQueueReader.Get() {
				f.IsQueueReaderActive.Set(false)
				return false
			}

			return true
		})
	}
}
