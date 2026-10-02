// Package hq4 consumes seeds from RabbitMQ queues which HQ publishes to, and acks them once finished.
// Outlinks and seencheck go through the HQ API, which HQv4 kept compatible with HQv3.
package hq4

import (
	"context"
	"sync"
	"time"

	"github.com/internetarchive/Zeno/v2/internal/pkg/config"
	"github.com/internetarchive/Zeno/v2/internal/pkg/log"
	"github.com/internetarchive/Zeno/v2/pkg/models"
	"github.com/internetarchive/gocrawlhq"
	"github.com/maypok86/otter"
	amqp "github.com/rabbitmq/amqp091-go"
)

type HQ struct {
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	finishCh  chan *models.Item
	produceCh chan *models.Item
	client    *gocrawlhq.Client
	rabbit    *SourceRabbit
	// deliveries of the seeds in the reactor, by item ID, acked once finished by map
	pending   map[string]amqp.Delivery
	pendingMu sync.Mutex
	// onFatal stops Zeno when the source can't continue
	onFatal        func()
	rabbitAddr     string
	routingKey     string
	HQKey          string
	HQSecret       string
	projectUUID    string
	HQAddress      string
	Timeout        int
	GZIPRequests   bool
	SeencheckURL   string
	seencheckCache *otter.Cache[string, seencheckCacheEntry]
}

// seencheckCacheEntry stores the seencheck result along with the source type
// ("asset" or "seed") so that a URL previously checked only as an asset will
// be re-checked when encountered as a seed.
type seencheckCacheEntry struct {
	seen   bool
	source string
}

var (
	once   sync.Once
	logger *log.FieldedLogger
)

func New(HQKey, HQSecret, projectUUID, HQAddress string, timeout, seencheckCacheSize int, gzipRequests bool, seencheckURL, rabbitAddr, routingKey string, onFatal func()) *HQ {

	h := &HQ{
		onFatal:      onFatal,
		rabbitAddr:   rabbitAddr,
		routingKey:   routingKey,
		HQKey:        HQKey,
		HQSecret:     HQSecret,
		projectUUID:  projectUUID,
		HQAddress:    HQAddress,
		Timeout:      timeout,
		GZIPRequests: gzipRequests,
		SeencheckURL: seencheckURL,
	}

	if seencheckCacheSize > 0 {
		cache, err := otter.MustBuilder[string, seencheckCacheEntry](seencheckCacheSize).Build()
		if err != nil {
			panic("failed to build seencheck cache: " + err.Error())
		}
		h.seencheckCache = &cache
	}

	return h
}

// Start initializes HQ async routines with the given input and output channels.
func (s *HQ) Start(finishChan, produceChan chan *models.Item) error {
	var done bool
	var startErr error

	logger = log.NewFieldedLogger(&log.Fields{
		"component": "hq",
	})

	once.Do(func() {
		done = true

		rabbit, err := NewSourceRabbit(s.rabbitAddr, s.projectUUID, s.routingKey, config.Get().HQBatchSize, s.onFatal)
		if err != nil {
			logger.Error("error configuring rabbit consumer", "err", err.Error(), "func", "hq.Start")
			startErr = err
			return
		}

		HQclient, err := gocrawlhq.Init(s.HQKey, s.HQSecret, s.projectUUID, s.HQAddress, "", s.Timeout, s.GZIPRequests)
		if err != nil {
			logger.Error("error initializing crawl HQ client", "err", err.Error(), "func", "hq.Start")
			startErr = err
			return
		}

		if err := rabbit.Start(); err != nil {
			logger.Error("error connecting to rabbit", "err", err.Error(), "func", "hq.Start")
			(*HQclient.WebsocketConn).Close()
			startErr = err
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		s.rabbit = rabbit
		s.pending = make(map[string]amqp.Delivery)
		s.wg = sync.WaitGroup{}
		s.ctx = ctx
		s.cancel = cancel
		s.finishCh = finishChan
		s.produceCh = produceChan
		s.client = HQclient

		if s.SeencheckURL != "" {
			s.client.AltSeencheckURL = s.SeencheckURL
		}

		s.wg.Add(4)
		go s.consumer()
		go s.producer()
		go s.finisher()
		go s.websocket()

		logger.Info("started")
	})

	if !done {
		return ErrHQAlreadyInitialized
	}

	return startErr
}

// Stop stops the global HQ and waits for all goroutines to finish. Finisher must be stopped first and Reactor must be frozen before stopping HQ.
func (s *HQ) Stop() {
	if s != nil && s.cancel != nil {
		s.cancel()
		s.wg.Wait()

		logger.Info("closing rabbit connection, unfinished seeds will be requeued by rabbit", "unfinished", s.pendingCount())
		s.rabbit.Stop()

		once = sync.Once{}
		if s.seencheckCache != nil {
			s.seencheckCache.Close()
			time.Sleep(1 * time.Second)
		}
		logger.Info("stopped")
	}
}

// Name returns the name of the source, used for logging and identification.
func (s *HQ) Name() string {
	return "hq4"
}
