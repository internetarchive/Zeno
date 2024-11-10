package crawl

import (
	"math"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/queue"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/gocrawlhq"
)

// This function connects to HQ's websocket and listen for messages.
// It also sends and "identify" message to the HQ to let it know that
// Zeno is connected. This "identify" message is sent every second and
// contains the crawler's stats and details.
func (c *Crawl) HQWebsocket() {
	var (
		// the "identify" message will be sent every second
		// to the crawl HQ
		identifyTicker = time.NewTicker(time.Second)
	)

	defer func() {
		identifyTicker.Stop()
	}()

	// send an "identify" message to the crawl HQ every second
	for {
		err := c.HQClient.Identify(&gocrawlhq.IdentifyMessage{
			Project:   c.HQProject,
			Job:       c.Job,
			IP:        utils.GetOutboundIP().String(),
			Hostname:  utils.GetHostname(),
			GoVersion: utils.GetVersion().GoVersion,
		})
		if err != nil {
			c.Log.WithFields(c.genLogFields(err, nil, map[string]interface{}{})).Error("error sending identify payload to crawl HQ, trying to reconnect..")

			err = c.HQClient.InitWebsocketConn()
			if err != nil {
				c.Log.WithFields(c.genLogFields(err, nil, map[string]interface{}{})).Error("error initializing websocket connection to crawl HQ")
			}
		}

		<-identifyTicker.C
	}
}

func (c *Crawl) HQProducer() {
	defer c.HQChannelsWg.Done()

	var (
		discoveredArray   = []gocrawlhq.URL{}
		mutex             = sync.Mutex{}
		terminateProducer = make(chan bool)
	)

	// the discoveredArray is sent to the crawl HQ every 10 seconds
	// or when it reaches a certain size
	go func() {
		HQLastSent := time.Now()

		for {
			select {
			case <-terminateProducer:
				// no need to lock the mutex here, because the producer channel
				// is already closed, so no other goroutine can write to the slice
				if len(discoveredArray) > 0 {
					for {
						err := c.HQClient.Add(discoveredArray, false)
						if err != nil {
							c.Log.WithFields(c.genLogFields(err, nil, map[string]interface{}{})).Error("error sending payload to crawl HQ, waiting 1s then retrying..")
							time.Sleep(time.Second)
							continue
						}
						break
					}
				}

				return
			default:
				mutex.Lock()
				if (len(discoveredArray) >= int(math.Ceil(float64(c.Workers.Count)/2)) || time.Since(HQLastSent) >= time.Second*10) && len(discoveredArray) > 0 {
					for {
						err := c.HQClient.Add(discoveredArray, false)
						if err != nil {
							c.Log.WithFields(c.genLogFields(err, nil, map[string]interface{}{})).Error("error sending payload to crawl HQ, waiting 1s then retrying..")
							time.Sleep(time.Second)
							continue
						}
						break
					}

					discoveredArray = []gocrawlhq.URL{}
					HQLastSent = time.Now()
				}
				mutex.Unlock()
			}
		}
	}()

	// listen to the discovered channel and add the URLs to the discoveredArray
	for discoveredItem := range c.HQProducerChannel {
		var via string

		if discoveredItem.ParentURL != nil {
			via = utils.URLToString(discoveredItem.ParentURL)
		}

		discoveredURL := gocrawlhq.URL{
			Value: utils.URLToString(discoveredItem.URL),
			Via:   via,
		}

		for i := uint64(0); i < discoveredItem.Hop; i++ {
			discoveredURL.Path += "L"
		}

		// The reason we are using a string instead of a bool is because
		// gob's encode/decode doesn't properly support booleans
		if discoveredItem.BypassSeencheck {
			for {
				err := c.HQClient.Add([]gocrawlhq.URL{discoveredURL}, true)
				if err != nil {
					c.Log.WithFields(c.genLogFields(err, nil, map[string]interface{}{
						"bypassSeencheck": discoveredItem.BypassSeencheck,
					})).Error("error sending payload to crawl HQ, waiting 1s then retrying..")
					time.Sleep(time.Second)
					continue
				}
				break
			}
			continue
		}

		mutex.Lock()
		discoveredArray = append(discoveredArray, discoveredURL)
		mutex.Unlock()
	}

	// if we are here, it means that the HQProducerChannel has been closed
	// so we need to send the last payload to the crawl HQ
	terminateProducer <- true
}

func (c *Crawl) HQConsumer() {
	for {
		c.HQConsumerState = "running"

		// This is on purpose evaluated every time,
		// because the value of workers will maybe change
		// during the crawl in the future (to be implemented)
		var HQBatchSize = int(c.Workers.Count)

		if c.Finished.Get() {
			c.HQConsumerState = "finished"
			c.Log.Error("crawl finished, stopping HQ consumer")
			break
		}

		// If HQContinuousPull is set to true, we will pull URLs from HQ continuously,
		// otherwise we will only pull URLs when needed (and when the crawl is not paused)
		for (c.Queue.GetStats().TotalElements > HQBatchSize && !c.HQContinuousPull) || c.Paused.Get() || c.Queue.HandoverOpen.Get() {
			c.HQConsumerState = "waiting"
			c.Log.Info("HQ producer waiting", "paused", c.Paused.Get(), "handoverOpen", c.Queue.HandoverOpen.Get(), "queueSize", c.Queue.GetStats().TotalElements)
			time.Sleep(time.Millisecond * 50)
			continue
		}

		// If a specific HQ batch size is set, use it
		if c.HQBatchSize != 0 {
			HQBatchSize = c.HQBatchSize
		}

		// get batch from crawl HQ
		c.HQConsumerState = "waitingOnFeed"
		var URLs []gocrawlhq.URL
		var err error
		if c.HQBatchConcurrency == 1 {
			URLs, err = c.HQClient.Get(HQBatchSize, c.HQStrategy)
			if err != nil {
				// c.Log.WithFields(c.genLogFields(err, nil, map[string]interface{}{
				// 	"batchSize": HQBatchSize,
				// 	"err":       err,
				// })).Debug("error getting new URLs from crawl HQ")
				continue
			}
		} else {
			var mu sync.Mutex
			var wg sync.WaitGroup
			batchSize := HQBatchSize / c.HQBatchConcurrency
			URLsChan := make(chan []gocrawlhq.URL, c.HQBatchConcurrency)

			// Start goroutines to get URLs from crawl HQ, each will request
			// HQBatchSize / HQConcurrentBatch URLs
			for i := 0; i < c.HQBatchConcurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					URLs, err := c.HQClient.Get(batchSize, c.HQStrategy)
					if err != nil {
						// c.Log.WithFields(c.genLogFields(err, nil, map[string]interface{}{
						// 	"batchSize": batchSize,
						// 	"err":       err,
						// })).Debug("error getting new URLs from crawl HQ")
						return
					}
					URLsChan <- URLs
				}()
			}

			// Wait for all goroutines to finish
			go func() {
				wg.Wait()
				close(URLsChan)
			}()

			// Collect all URLs from the channels
			for URLsFromChan := range URLsChan {
				mu.Lock()
				URLs = append(URLs, URLsFromChan...)
				mu.Unlock()
			}
		}
		c.HQConsumerState = "feedCompleted"

		// send all URLs received in the batch to the queue
		var items = make([]*queue.Item, 0, len(URLs))
		if len(URLs) > 0 {
			for _, URL := range URLs {
				c.HQConsumerState = "urlParse"
				newURL, err := url.Parse(URL.Value)
				if err != nil {
					c.Log.WithFields(c.genLogFields(err, nil, map[string]interface{}{
						"url":       URL.Value,
						"batchSize": HQBatchSize,
						"err":       err,
					})).Error("unable to parse URL received from crawl HQ, discarding")
					continue
				}

				c.HQConsumerState = "newItem"
				newItem, err := queue.NewItem(newURL, nil, "seed", uint64(strings.Count(URL.Path, "L")), URL.ID, false)
				if err != nil {
					c.Log.WithFields(c.genLogFields(err, newURL, map[string]interface{}{
						"url":       URL.Value,
						"batchSize": HQBatchSize,
						"err":       err,
					})).Error("unable to create new item from URL received from crawl HQ, discarding")
					continue
				}

				c.HQConsumerState = "append"
				items = append(items, newItem)
			}
		}

		c.HQConsumerState = "enqueue"
		err = c.Queue.BatchEnqueue(items...)
		if err != nil {
			c.Log.Error("unable to enqueue URL batch received from crawl HQ, discarding", "error", err)
			continue
		}
	}
}

func (c *Crawl) HQFinisher() {
	defer c.HQChannelsWg.Done()

	var (
		finishedArray       = []gocrawlhq.URL{}
		locallyCrawledTotal int
	)

	for finishedItem := range c.HQFinishedChannel {
		if finishedItem.ID == "" {
			c.Log.WithFields(c.genLogFields(nil, finishedItem.URL, nil)).Warn("URL has no ID, discarding")
			continue
		}

		locallyCrawledTotal += int(finishedItem.LocallyCrawled)
		finishedArray = append(finishedArray, gocrawlhq.URL{ID: finishedItem.ID, Value: utils.URLToString(finishedItem.URL)})

		if len(finishedArray) == int(math.Ceil(float64(c.Workers.Count)/2)) {
			for {
				err := c.HQClient.Delete(finishedArray, locallyCrawledTotal)
				if err != nil {
					c.Log.WithFields(c.genLogFields(err, nil, map[string]interface{}{
						"finishedArray": finishedArray,
					})).Error("error submitting finished urls to crawl HQ. retrying in one second...")
					time.Sleep(time.Second)
					continue
				}
				break
			}

			finishedArray = []gocrawlhq.URL{}
			locallyCrawledTotal = 0
		}
	}

	// send remaining finished URLs
	if len(finishedArray) > 0 {
		for {
			err := c.HQClient.Delete(finishedArray, locallyCrawledTotal)
			if err != nil {
				c.Log.WithFields(c.genLogFields(err, nil, map[string]interface{}{
					"finishedArray": finishedArray,
				})).Error("error submitting finished urls to crawl HQ. retrying in one second...")
				time.Sleep(time.Second)
				continue
			}
			break
		}
	}
}

func (c *Crawl) HQSeencheckURLs(URLs []*url.URL) (seencheckedBatch []*url.URL, err error) {
	var (
		discoveredURLs []gocrawlhq.URL
	)

	for _, URL := range URLs {
		discoveredURLs = append(discoveredURLs, gocrawlhq.URL{
			Value: utils.URLToString(URL),
			Type:  "asset",
		})
	}

	outputURLs, err := c.HQClient.Seencheck(discoveredURLs)
	if err != nil {
		c.Log.WithFields(c.genLogFields(err, nil, map[string]interface{}{
			"batchLen": len(URLs),
			"urls":     discoveredURLs,
		})).Error("error sending seencheck payload to crawl HQ")
		return seencheckedBatch, err
	}

	if outputURLs != nil {
		for _, URL := range outputURLs {
			// the returned payload only contain new URLs to be crawled by Zeno
			newURL, err := url.Parse(URL.Value)
			if err != nil {
				c.Log.WithFields(c.genLogFields(err, URL, map[string]interface{}{
					"batchLen": len(URLs),
				})).Error("error parsing URL from HQ seencheck response")
				return seencheckedBatch, err
			}

			seencheckedBatch = append(seencheckedBatch, newURL)
		}
	}

	return seencheckedBatch, nil
}

// returns:
//   - bool: true if the URL is new, false if it has been seen before
//   - error: if there's an error sending the payload to crawl HQ
//
// NOTE: if there's an error, the URL is considered new
func (c *Crawl) HQSeencheckURL(URL *url.URL) (bool, error) {
	discoveredURL := gocrawlhq.URL{
		Value: utils.URLToString(URL),
		Type:  "asset",
	}

	outputURLs, err := c.HQClient.Seencheck([]gocrawlhq.URL{discoveredURL})
	if err != nil {
		c.Log.Error("error sending seencheck payload to crawl HQ", "err", err, "url", utils.URLToString(URL))
		return true, err // return true, don't discard the URL if there's an error
	}

	if outputURLs != nil {
		for _, URL := range outputURLs {
			// the returned payload only contain new URLs to be crawled by Zeno
			if URL.Value == discoveredURL.Value {
				return true, nil
			}
		}
	}

	return false, nil
}
