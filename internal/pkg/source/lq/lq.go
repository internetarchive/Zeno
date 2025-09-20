package lq

import (
	"context"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/pkg/models"
)

type LQ struct {
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	finishCh  chan *models.Item
	produceCh chan *models.Item
	client    *lqClient
}

var (
	once   sync.Once
	logger *log.FieldedLogger
)

func New() *LQ {
	return &LQ{}
}

func (s *LQ) Start(finishChan, produceChan chan *models.Item) error {
	var done bool
	var startErr error

	logger = log.NewFieldedLogger(&log.Fields{
		"component": "lq",
	})

	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		LQclient, err := initClient(config.Get().Job)
		if err != nil {
			logger.Error("error initializing crawl LQ client", "err", err.Error(), "func", "lq.Start")
			cancel()
			done = true
			startErr = err
			return
		}

		s.wg = sync.WaitGroup{}
		s.ctx = ctx
		s.cancel = cancel
		s.finishCh = finishChan
		s.produceCh = produceChan
		s.client = LQclient

		s.wg.Add(3)
		go s.consumer()
		go s.producer()
		go s.finisher()

		logger.Info("started")

		done = true
	})

	if !done {
		return ErrLQAlreadyInitialized
	}

	return startErr
}

func (s *LQ) Stop() {
	if s != nil {
		s.cancel()
		s.wg.Wait()
		seedsToReset := reactor.GetStateTable()
		for _, seed := range seedsToReset {
			if err := s.client.resetURL(s.ctx, seed); err != nil {
				logger.Error("error while reseting", "id", seed, "err", err)
			}
			logger.Debug("reset seed", "id", seed)
		}
		once = sync.Once{}
		logger.Info("stopped")
	}
}

func (s *LQ) Name() string {
	return "Local Queue"
}
