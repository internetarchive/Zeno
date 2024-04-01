package crawl

import (
	"math"
	"net/url"
	"strings"
	"sync"
	"time"

	"git.archive.org/wb/gocrawlhq"
	"github.com/internetarchive/Zeno/internal/pkg/frontier"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/sirupsen/logrus"
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
			logrus.WithFields(c.genLogFields(err, nil, nil)).Errorln("error sending identify payload to crawl HQ, trying to reconnect..")
			err = c.HQClient.InitWebsocketConn()
			if err != nil {
				logrus.WithFields(c.genLogFields(err, nil, nil)).Errorln("error initializing websocket connection to crawl HQ")
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
						_, err := c.HQClient.Discovered(discoveredArray, "seed", false, false)
						if err != nil {
							logrus.WithFields(c.genLogFields(err, nil, nil)).Errorln("error sending payload to crawl HQ, waiting 1s then retrying..")
							time.Sleep(time.Second)
							continue
						}
						break
					}
				}

				return
			default:
				mutex.Lock()
				if (len(discoveredArray) >= int(math.Ceil(float64(c.Workers)/2)) || time.Since(HQLastSent) >= time.Second*10) && len(discoveredArray) > 0 {
					for {
						_, err := c.HQClient.Discovered(discoveredArray, "seed", false, false)
						if err != nil {
							logrus.WithFields(c.genLogFields(err, nil, nil)).Errorln("error sending payload to crawl HQ, waiting 1s then retrying..")
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

		if discoveredItem.ParentItem != nil {
			via = utils.URLToString(discoveredItem.ParentItem.URL)
		}

		discoveredURL := gocrawlhq.URL{
			Value: utils.URLToString(discoveredItem.URL),
			Via:   via,
		}

		for i := 0; uint8(i) < discoveredItem.Hop; i++ {
			discoveredURL.Path += "L"
		}

		if *discoveredItem.BypassSeencheck {
			for {
				_, err := c.HQClient.Discovered([]gocrawlhq.URL{discoveredURL}, "seed", true, false)
				if err != nil {
					logrus.WithFields(c.genLogFields(err, nil, nil)).Errorln("error sending payload to crawl HQ, waiting 1s then retrying..")
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
		// This is on purpose evaluated every time,
		// because the value of workers will maybe change
		// during the crawl in the future (to be implemented)
		var HQBatchSize = int(math.Ceil(float64(c.Workers) / 2))

		if c.Finished.Get() {
			break
		}

		if c.Paused.Get() {
			time.Sleep(time.Second)
		}

		// If HQContinuousPull is set to true, we will pull URLs from HQ
		// continuously, otherwise we will only pull URLs when needed
		if !c.HQContinuousPull {
			if c.ActiveWorkers.Value() >= int64(c.Workers-(c.Workers/10)) {
				time.Sleep(time.Millisecond * 100)
				continue
			}
		}

		// If a specific HQ batch size is set, use it
		if c.HQBatchSize != 0 {
			HQBatchSize = c.HQBatchSize
		}

		// get batch from crawl HQ
		batch, err := c.HQClient.Feed(HQBatchSize, c.HQStrategy)
		if err != nil {
			logrus.WithFields(c.genLogFields(err, nil, map[string]interface{}{
				"batchSize": HQBatchSize,
			})).Debugln("error getting new URLs from crawl HQ")
		}

		// send all URLs received in the batch to the frontier
		for _, URL := range batch.URLs {
			newURL, err := url.Parse(URL.Value)
			if err != nil {
				logrus.WithFields(c.genLogFields(err, nil, map[string]interface{}{
					"batchSize": HQBatchSize,
				})).Errorln("unable to parse URL received from crawl HQ, discarding")
			}

			c.Frontier.PushChan <- frontier.NewItem(newURL, nil, "seed", uint8(strings.Count(URL.Path, "L")), URL.ID, utils.Pointer(false))
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
			logWarning.WithFields(c.genLogFields(nil, finishedItem.URL, nil)).Warnln("URL has no ID, discarding")
			continue
		}

		locallyCrawledTotal += int(finishedItem.LocallyCrawled)
		finishedArray = append(finishedArray, gocrawlhq.URL{ID: finishedItem.ID, Value: utils.URLToString(finishedItem.URL)})

		if len(finishedArray) == int(math.Ceil(float64(c.Workers)/2)) {
			for {
				_, err := c.HQClient.Finished(finishedArray, locallyCrawledTotal)
				if err != nil {
					logError.WithFields(c.genLogFields(err, nil, map[string]interface{}{
						"finishedArray": finishedArray,
					})).Errorln("error submitting finished urls to crawl HQ. retrying in one second...")
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
			_, err := c.HQClient.Finished(finishedArray, locallyCrawledTotal)
			if err != nil {
				logError.WithFields(c.genLogFields(err, nil, map[string]interface{}{
					"finishedArray": finishedArray,
				})).Errorln("error submitting finished urls to crawl HQ. retrying in one second...")
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
		})
	}

	discoveredResponse, err := c.HQClient.Discovered(discoveredURLs, "asset", false, true)
	if err != nil {
		logError.WithFields(c.genLogFields(err, nil, map[string]interface{}{
			"batchLen": len(URLs),
			"urls":     discoveredURLs,
		})).Errorln("error sending seencheck payload to crawl HQ")
		return seencheckedBatch, err
	}

	if discoveredResponse.URLs != nil {
		for _, URL := range discoveredResponse.URLs {
			// the returned payload only contain new URLs to be crawled by Zeno
			newURL, err := url.Parse(URL.Value)
			if err != nil {
				logError.WithFields(c.genLogFields(err, URL, map[string]interface{}{
					"batchLen": len(URLs),
				})).Errorln("error parsing URL from HQ seencheck response")
				return seencheckedBatch, err
			}

			seencheckedBatch = append(seencheckedBatch, newURL)
		}
	}

	return seencheckedBatch, nil
}

func (c *Crawl) HQSeencheckURL(URL *url.URL) (bool, error) {
	discoveredURL := gocrawlhq.URL{
		Value: utils.URLToString(URL),
	}

	discoveredResponse, err := c.HQClient.Discovered([]gocrawlhq.URL{discoveredURL}, "asset", false, true)
	if err != nil {
		logrus.WithFields(c.genLogFields(err, URL, nil)).Errorln("error sending seencheck payload to crawl HQ")
		return false, err
	}

	if discoveredResponse.URLs != nil {
		for _, URL := range discoveredResponse.URLs {
			if URL.Value == discoveredURL.Value {
				return false, nil
			}
		}
	}

	// didn't find the URL in the HQ, so it's new and has been added to HQ's seencheck database
	return true, nil
}
