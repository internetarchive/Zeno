package archiver

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gabriel-vasile/mimetype"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/general"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/headless"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/ratelimiter"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/internetarchive/Zeno/pkg/models"
	warc "github.com/internetarchive/gowarc"
)

func init() {
	// We intentionally set the limit to 0 to disable the limit on the number of bytes the
	// mimetype detection can accept. We limit the number of bytes that we will give to it
	// in the processBody function instead.
	mimetype.SetLimit(0)
}

type archiver struct {
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	inputCh  chan *models.Item
	outputCh chan *models.Item

	Client          *warc.CustomHTTPClient
	ClientWithProxy *warc.CustomHTTPClient
}

var (
	globalArchiver      *archiver
	globalBucketManager *ratelimiter.BucketManager
	once                sync.Once
	logger              *log.FieldedLogger
)

// Start initializes the internal archiver structure, start the WARC writer and start routines, should only be called once and returns an error if called more than once
func Start(inputChan, outputChan chan *models.Item) error {
	logger = log.NewFieldedLogger(&log.Fields{
		"component": "archiver",
	})

	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		globalArchiver = &archiver{
			ctx:      ctx,
			cancel:   cancel,
			inputCh:  inputChan,
			outputCh: outputChan,
		}
		if !config.Get().DisableRateLimit {
			globalBucketManager = ratelimiter.NewBucketManager(ctx,
				config.Get().WorkersCount*config.Get().MaxConcurrentAssets, // maxBuckets
				config.Get().RateLimitCapacity,
				config.Get().RateLimitRefillRate,
				config.Get().RateLimitCleanupFrequency,
			)
			logger.Info("bucket manager started")
		}
		if config.Get().Headless {
			headless.Start()
			logger.Info("headless browser started")
		}

		logger.Debug("initialized")

		// Setup WARC writing HTTP clients
		startWARCWriter()

		logger.Debug("WARC writer started")

		for i := 0; i < config.Get().WorkersCount; i++ {
			globalArchiver.wg.Add(1)
			go globalArchiver.worker(strconv.Itoa(i))
		}

		logger.Info("started")
	})

	if globalArchiver == nil {
		return ErrArchiverAlreadyInitialized
	}

	return nil
}

// Stop stops the archiver routines and the WARC writer
func Stop() {
	if globalArchiver != nil {
		globalArchiver.cancel()
		globalArchiver.wg.Wait()

		// Wait for the WARC writing to finish
		stopLocalWatcher := make(chan struct{})
		go func() {
			for {
				select {
				case <-stopLocalWatcher:
					return
				case <-time.After(1 * time.Second):
					logger.Debug("waiting for WARC writing to finish", "queue_size", GetWARCWritingQueueSize(), "bytes_written", humanize.Bytes(uint64(warc.DataTotal.Load())))
				}
			}
		}()
		globalArchiver.Client.WaitGroup.Wait()
		stopLocalWatcher <- struct{}{}
		logger.Debug("WARC writing finished")
		globalArchiver.Client.Close()
		if globalArchiver.ClientWithProxy != nil {
			globalArchiver.ClientWithProxy.WaitGroup.Wait()
			globalArchiver.ClientWithProxy.Close()
		}

		logger.Info("stopped")
	}
	if headless.HeadlessBrowser != nil {
		logger.Debug("closing headless browser")
		headless.Close()
		logger.Info("closed headless browser")
	}
	if globalBucketManager != nil {
		logger.Debug("closing bucket manager")
		globalBucketManager.Close()
		logger.Info("closed bucket manager")
	}
}

func (a *archiver) worker(workerID string) {
	defer a.wg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "archiver.worker",
		"worker_id": workerID,
	})

	defer logger.Debug("worker stopped")

	// Subscribe to the pause controler
	controlChans := pause.Subscribe()
	defer pause.Unsubscribe(controlChans)

	stats.ArchiverRoutinesIncr()
	defer stats.ArchiverRoutinesDecr()

	for {
		select {
		case <-a.ctx.Done():
			logger.Debug("shutting down")
			return
		case <-controlChans.PauseCh:
			logger.Debug("received pause event")
			controlChans.ResumeCh <- struct{}{}
			logger.Debug("received resume event")
		case seed, ok := <-a.inputCh:
			if ok {
				logger.Debug("received seed", "seed", seed.GetShortID(), "depth", seed.GetDepth(), "hops", seed.GetURL().GetHops())

				if err := seed.CheckConsistency(); err != nil {
					panic(fmt.Sprintf("seed consistency check failed with err: %s, seed id %s", err.Error(), seed.GetShortID()))
				}

				if seed.GetStatus() != models.ItemPreProcessed && seed.GetStatus() != models.ItemGotRedirected && seed.GetStatus() != models.ItemGotChildren {
					logger.Debug("skipping seed", "seed", seed.GetShortID(), "depth", seed.GetDepth(), "hops", seed.GetURL().GetHops(), "status", seed.GetStatus())
				} else {
					archive(workerID, seed)
				}

				select {
				case <-a.ctx.Done():
					logger.Debug("aborting seed due to stop", "seed", seed.GetShortID(), "depth", seed.GetDepth(), "hops", seed.GetURL().GetHops())
					return
				case a.outputCh <- seed:
				}
			}
		}
	}
}

func archive(workerID string, seed *models.Item) {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "archiver.archive",
		"worker_id": workerID,
	})

	var (
		guard = make(chan struct{}, config.Get().MaxConcurrentAssets)
		wg    sync.WaitGroup
	)

	items, err := seed.GetNodesAtLevel(seed.GetMaxDepth())
	if err != nil {
		logger.Error("unable to get nodes at level", "err", err.Error(), "seed_id", seed.GetShortID())
		panic(err)
	}

	var client *warc.CustomHTTPClient
	if config.Get().Proxy != "" {
		client = globalArchiver.ClientWithProxy
	} else {
		client = globalArchiver.Client
	}

	for i := range items {
		if items[i].GetStatus() != models.ItemPreProcessed {
			logger.Debug("skipping item", "item_id", items[i].GetShortID(), "status", items[i].GetStatus())
			continue
		}

		guard <- struct{}{}

		wg.Add(1)

		if config.Get().Headless {
			go headless.ArchiveItem(items[i], &wg, guard, globalBucketManager, client)
		} else {
			go general.ArchiveItem(items[i], &wg, guard, globalBucketManager, client)
		}

	}

	// Wait for all goroutines to finish
	wg.Wait()
}
