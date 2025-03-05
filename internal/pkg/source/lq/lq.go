package lq

import (
	"context"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/internetarchive/Zeno/pkg/models"
)

type lq struct {
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	finishCh  chan *models.Item
	produceCh chan *models.Item
	client    *LQClient
}

var (
	globalLQ *lq
	once     sync.Once
	logger   *log.FieldedLogger
)

func Start(finishChan, produceChan chan *models.Item) error {
	var done bool
	var startErr error

	log.Start()
	logger = log.NewFieldedLogger(&log.Fields{
		"component": "lq",
	})

	stats.Init()

	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		LQclient, err := Init(config.Get().Job)
		if err != nil {
			logger.Error("error initializing crawl LQ client", "err", err.Error(), "func", "lq.Start")
			cancel()
			done = true
			startErr = err
			return
		}

		globalLQ = &lq{
			wg:        sync.WaitGroup{},
			ctx:       ctx,
			cancel:    cancel,
			finishCh:  finishChan,
			produceCh: produceChan,
			client:    LQclient,
		}

		globalLQ.wg.Add(3)
		go consumer()
		go producer()
		go finisher()

		logger.Info("started")

		done = true
	})

	if !done {
		return ErrLQAlreadyInitialized
	}

	return startErr
}

func Stop() {
	if globalLQ != nil {
		globalLQ.cancel()
		globalLQ.wg.Wait()
		seedsToReset := reactor.GetStateTable()
		for _, seed := range seedsToReset {
			if err := globalLQ.client.ResetURL(context.TODO(), seed); err != nil {
				logger.Error("error while reseting", "id", seed, "err", err)
			}
			logger.Debug("reset seed", "id", seed)
		}
		once = sync.Once{}
		logger.Info("stopped")
	}
}
