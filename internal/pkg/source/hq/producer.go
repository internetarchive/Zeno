package hq

// import (
// 	"math"
// 	"sync"
// 	"time"

// 	"github.com/internetarchive/Zeno/internal/pkg/log"
// 	"github.com/internetarchive/gocrawlhq"
// )

// func producer() {
// 	var (
// 		wg     sync.WaitGroup
// 		logger = log.NewFieldedLogger(&log.Fields{
// 			"component": "hq/producer",
// 		})
// 	)

// 	// the discoveredArray is sent to the crawl HQ every 10 seconds
// 	// or when it reaches a certain size
// 	go func() {
// 		HQLastSent := time.Now()

// 		for {
// 			select {
// 			case <-globalHQ.ctx.Done():
// 				// no need to lock the mutex here, because the producer channel
// 				// is already closed, so no other goroutine can write to the slice
// 				if len(discoveredArray) > 0 {
// 					for {
// 						err := globalHQ.client.Add(discoveredArray, false)
// 						if err != nil {
// 							logger.Error("error sending payload to crawl HQ, waiting 1s then retrying..")
// 							time.Sleep(time.Second)
// 							continue
// 						}
// 						break
// 					}
// 				}

// 				return
// 			default:
// 				mutex.Lock()
// 				if (len(discoveredArray) >= int(math.Ceil(float64(c.Workers.Count)/2)) || time.Since(HQLastSent) >= time.Second*10) && len(discoveredArray) > 0 {
// 					for {
// 						err := c.HQClient.Add(discoveredArray, false)
// 						if err != nil {
// 							c.Log.WithFields(c.genLogFields(err, nil, map[string]interface{}{})).Error("error sending payload to crawl HQ, waiting 1s then retrying..")
// 							time.Sleep(time.Second)
// 							continue
// 						}
// 						break
// 					}

// 					discoveredArray = []gocrawlhq.URL{}
// 					HQLastSent = time.Now()
// 				}
// 				mutex.Unlock()
// 			}
// 		}
// 	}()

// 	// listen to the discovered channel and add the URLs to the discoveredArray
// 	for discoveredItem := range c.HQProducerChannel {
// 		var via string

// 		if discoveredItem.ParentURL != nil {
// 			via = utils.URLToString(discoveredItem.ParentURL)
// 		}

// 		discoveredURL := gocrawlhq.URL{
// 			Value: utils.URLToString(discoveredItem.URL),
// 			Via:   via,
// 		}

// 		for i := uint64(0); i < discoveredItem.Hop; i++ {
// 			discoveredURL.Path += "L"
// 		}

// 		// The reason we are using a string instead of a bool is because
// 		// gob's encode/decode doesn't properly support booleans
// 		if discoveredItem.BypassSeencheck {
// 			for {
// 				err := c.HQClient.Add([]gocrawlhq.URL{discoveredURL}, true)
// 				if err != nil {
// 					c.Log.WithFields(c.genLogFields(err, nil, map[string]interface{}{
// 						"bypassSeencheck": discoveredItem.BypassSeencheck,
// 					})).Error("error sending payload to crawl HQ, waiting 1s then retrying..")
// 					time.Sleep(time.Second)
// 					continue
// 				}
// 				break
// 			}
// 			continue
// 		}

// 		mutex.Lock()
// 		discoveredArray = append(discoveredArray, discoveredURL)
// 		mutex.Unlock()
// 	}

// 	// if we are here, it means that the HQProducerChannel has been closed
// 	// so we need to send the last payload to the crawl HQ
// 	terminateProducer <- true
// }
