// Package hq provides a way to interact with the HQv3 API and consumes, produces and mark items as finished asynchronusly.
package hq

import (
	"context"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
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

// Start initializes HQ async routines with the given input and output channels.
func Start(finishChan, produceChan chan *models.Item) error {
	var done bool
	var startErr error

	log.Start()
	logger = log.NewFieldedLogger(&log.Fields{
		"component": "hq",
	})

	stats.Init()

	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		HQclient, err := gocrawlhq.Init(config.Get().HQKey, config.Get().HQSecret, config.Get().HQProject, config.Get().HQAddress, "")
		if err != nil {
			logger.Error("error initializing crawl HQ client", "err", err.Error(), "func", "hq.Start")
			cancel()
			done = true
			startErr = err
			return
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

		logger.Info("started")

		done = true
	})

	if !done {
		return ErrHQAlreadyInitialized
	}

	return startErr
}

// Stop stops the global HQ and waits for all goroutines to finish. Finisher must be stopped first and Reactor must be frozen before stopping HQ.
func Stop() {
	if globalHQ != nil {
		globalHQ.cancel()
		globalHQ.wg.Wait()
		seedsToReset := reactor.GetStateTable()
		for _, seed := range seedsToReset {
			if err := globalHQ.client.ResetURL(globalHQ.ctx, seed); err != nil {
				logger.Error("error while reseting", "id", seed, "err", err)
			}
			logger.Debug("reset seed", "id", seed)
		}
		once = sync.Once{}
		logger.Info("stopped")
	}
}
