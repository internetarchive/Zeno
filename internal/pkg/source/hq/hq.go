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
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	finishCh  chan *models.Item
	produceCh chan *models.Item
	client    *gocrawlhq.Client
}

var (
	globalHQ *hq
	once     sync.Once
	logger   *log.FieldedLogger
)

func Start(finishChan, produceChan chan *models.Item) error {
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
			wg:        sync.WaitGroup{},
			ctx:       ctx,
			cancel:    cancel,
			finishCh:  finishChan,
			produceCh: produceChan,
			client:    HQclient,
		}

		globalHQ.wg.Add(3)
		go consumer()
		go producer()
		go finisher()
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
		once = sync.Once{}
		logger.Info("stopped")
	}
}

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
