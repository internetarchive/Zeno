package hq

import (
	"context"
	"os"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gocrawlhq"
)

type hq struct {
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	inputCh chan *models.Item
	client  *gocrawlhq.Client
}

var (
	globalHQ *hq
	once     sync.Once
	logger   *log.FieldedLogger
)

func Start(inputChan chan *models.Item) error {
	var done bool

	log.Start()
	logger = log.NewFieldedLogger(&log.Fields{
		"component": "hq",
	})

	stats.Init()

	once.Do(func() {
		var err error

		ctx, cancel := context.WithCancel(context.Background())
		HQclient, err := gocrawlhq.Init(config.Get().HQKey, config.Get().HQSecret, config.Get().HQProject, config.Get().HQAddress, "")
		if err != nil {
			logger.Error("error initializing crawl HQ client", "err", err.Error(), "func", "hq.Start")
			os.Exit(1)
		}

		globalHQ = &hq{
			wg:      sync.WaitGroup{},
			ctx:     ctx,
			cancel:  cancel,
			inputCh: inputChan,
			client:  HQclient,
		}

		globalHQ.wg.Add(1)
		go consumer()
		// go producer()
		// go finisher()
		done = true
	})

	if !done {
		return ErrHQAlreadyInitialized
	}

	return nil
}

func Stop() {
	if globalHQ != nil {
		globalHQ.cancel()
		globalHQ.wg.Wait()
		logger.Info("stopped")
	}
}

// func HQProducer() {
// 	defer c.HQChannelsWg.Done()

// 	var (
// 		discoveredArray   = []gocrawlhq.URL{}
// 		mutex             = sync.Mutex{}
// 		terminateProducer = make(chan bool)
// 	)

// 	// the discoveredArray is sent to the crawl HQ every 10 seconds
// 	// or when it reaches a certain size
// 	go func() {
// 		HQLastSent := time.Now()

// 		for {
// 			select {
// 			case <-terminateProducer:
// 				// no need to lock the mutex here, because the producer channel
// 				// is already closed, so no other goroutine can write to the slice
// 				if len(discoveredArray) > 0 {
// 					for {
// 						err := c.HQClient.Add(discoveredArray, false)
// 						if err != nil {
// 							c.Log.WithFields(c.genLogFields(err, nil, map[string]interface{}{})).Error("error sending payload to crawl HQ, waiting 1s then retrying..")
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

// func HQFinisher() {
// 	defer c.HQChannelsWg.Done()

// 	var (
// 		finishedArray       = []gocrawlhq.URL{}
// 		locallyCrawledTotal int
// 	)

// 	for finishedItem := range c.HQFinishedChannel {
// 		if finishedItem.ID == "" {
// 			c.Log.WithFields(c.genLogFields(nil, finishedItem.URL, nil)).Warn("URL has no ID, discarding")
// 			continue
// 		}

// 		locallyCrawledTotal += int(finishedItem.LocallyCrawled)
// 		finishedArray = append(finishedArray, gocrawlhq.URL{ID: finishedItem.ID, Value: utils.URLToString(finishedItem.URL)})

// 		if len(finishedArray) == int(math.Ceil(float64(c.Workers.Count)/2)) {
// 			for {
// 				err := c.HQClient.Delete(finishedArray, locallyCrawledTotal)
// 				if err != nil {
// 					c.Log.WithFields(c.genLogFields(err, nil, map[string]interface{}{
// 						"finishedArray": finishedArray,
// 					})).Error("error submitting finished urls to crawl HQ. retrying in one second...")
// 					time.Sleep(time.Second)
// 					continue
// 				}
// 				break
// 			}

// 			finishedArray = []gocrawlhq.URL{}
// 			locallyCrawledTotal = 0
// 		}
// 	}

// 	// send remaining finished URLs
// 	if len(finishedArray) > 0 {
// 		for {
// 			err := c.HQClient.Delete(finishedArray, locallyCrawledTotal)
// 			if err != nil {
// 				c.Log.WithFields(c.genLogFields(err, nil, map[string]interface{}{
// 					"finishedArray": finishedArray,
// 				})).Error("error submitting finished urls to crawl HQ. retrying in one second...")
// 				time.Sleep(time.Second)
// 				continue
// 			}
// 			break
// 		}
// 	}
// }
