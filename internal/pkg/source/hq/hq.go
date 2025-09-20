// Package hq provides a way to interact with the HQv3 API and consumes, produces and mark items as finished asynchronusly.
package hq

import (
	"context"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gocrawlhq"
)

type HQ struct {
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	finishCh  chan *models.Item
	produceCh chan *models.Item
	client    *gocrawlhq.Client
	HQKey     string
	HQSecret  string
	HQProject string
	HQAddress string
}

var (
	logger *log.FieldedLogger
)

func New(HQKey, HQSecret, HQProject, HQAddress string) *HQ {
	return &HQ{
		HQKey:     HQKey,
		HQSecret:  HQSecret,
		HQProject: HQProject,
		HQAddress: HQAddress,
	}
}

// Start initializes HQ async routines with the given input and output channels.
func (s *HQ) Start(finishChan, produceChan chan *models.Item) error {
	logger = log.NewFieldedLogger(&log.Fields{
		"component": "hq",
	})

	ctx, cancel := context.WithCancel(context.Background())
	HQclient, err := gocrawlhq.Init(s.HQKey, s.HQSecret, s.HQProject, s.HQAddress, "", 5)
	if err != nil {
		logger.Error("error initializing crawl HQ client", "err", err.Error(), "func", "hq.Start")
		cancel()
		return err
	}

	s.wg = sync.WaitGroup{}
	s.ctx = ctx
	s.cancel = cancel
	s.finishCh = finishChan
	s.produceCh = produceChan
	s.client = HQclient

	s.wg.Add(4)
	go s.consumer()
	go s.producer()
	go s.finisher()
	go s.websocket()

	logger.Info("started", "project", s.HQProject)

	return nil
}

// Stop stops the global HQ and waits for all goroutines to finish. Finisher must be stopped first and Reactor must be frozen before stopping HQ.
func (s *HQ) Stop() {
	if s != nil {
		s.cancel()
		s.wg.Wait()
		seedsToReset := reactor.GetStateTable()
		for _, seed := range seedsToReset {
			if err := s.client.ResetURL(s.ctx, seed); err != nil {
				logger.Error("error while reseting", "id", seed, "err", err)
			}
			logger.Debug("reset seed", "id", seed)
		}
		logger.Info("stopped")
	}
}

// Name returns the name of the source, used for logging and identification.
func (s *HQ) Name() string {
	return "hq"
}
