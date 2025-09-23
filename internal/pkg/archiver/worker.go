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

	var onceErr error

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
		if err := startWARCWriter(); err != nil {
			onceErr = err
			return
		}

		logger.Debug("WARC writer started")

		for i := 0; i < config.Get().WorkersCount; i++ {
			globalArchiver.wg.Add(1)
			go globalArchiver.worker(strconv.Itoa(i))
		}

		logger.Info("started")
	})

	if onceErr != nil {
		return onceErr
	}

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

	// Cancel config related contexts
	// e.g. the goroutine that watches for exclusions file changes
	config.Get().Cancel()
	logger.Info("stopped config related contexts")
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

	// Separate main page from assets
	var mainItems, assetItems []*models.Item
	for i := range items {
		if items[i].GetStatus() != models.ItemPreProcessed {
			logger.Debug("skipping item", "item_id", items[i].GetShortID(), "status", items[i].GetStatus())
			continue
		}

		// Check if this is the main page (depth 0) or an asset
		if items[i].GetDepth() == 0 {
			mainItems = append(mainItems, items[i])
		} else {
			assetItems = append(assetItems, items[i])
		}
	}

	// Archive main pages first
	for _, item := range mainItems {
		guard <- struct{}{}
		wg.Add(1)

		if config.Get().Headless {
			go headless.ArchiveItem(item, &wg, guard, globalBucketManager, client)
		} else {
			go general.ArchiveItem(item, &wg, guard, globalBucketManager, client)
		}
	}

	// Archive assets with optional timeout
	if len(assetItems) > 0 {
		archiveAssetsWithTimeout(workerID, assetItems, &wg, guard, globalBucketManager, client)
	}

	// Wait for all goroutines to finish
	wg.Wait()
}

// archiveAssetsWithTimeout archives assets with optional timeout
func archiveAssetsWithTimeout(workerID string, assetItems []*models.Item, wg *sync.WaitGroup, guard chan struct{}, bucketManager *ratelimiter.BucketManager, client *warc.CustomHTTPClient) {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "archiver.archiveAssetsWithTimeout",
		"worker_id": workerID,
	})

	cfg := config.Get()
	
	// If no timeout is configured, archive all assets normally
	if cfg.AssetsArchivingTimeout == 0 {
		for _, item := range assetItems {
			guard <- struct{}{}
			wg.Add(1)

			if cfg.Headless {
				go headless.ArchiveItem(item, wg, guard, bucketManager, client)
			} else {
				go general.ArchiveItem(item, wg, guard, bucketManager, client)
			}
		}
		return
	}

	// Archive assets with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.AssetsArchivingTimeout)
	defer cancel()

	assetWg := sync.WaitGroup{}
	archivedCount := 0
	skippedCount := 0

	logger.Debug("starting assets archiving with timeout", "timeout", cfg.AssetsArchivingTimeout, "total_assets", len(assetItems))

	for _, item := range assetItems {
		select {
		case <-ctx.Done():
			// Timeout reached, skip remaining assets
			logger.Debug("assets archiving timeout reached", "archived", archivedCount, "skipped", len(assetItems)-archivedCount)
			skippedCount = len(assetItems) - archivedCount
			
			// Set status of remaining items to indicate they were skipped due to timeout
			for j := archivedCount; j < len(assetItems); j++ {
				assetItems[j].SetStatus(models.ItemCompleted) // Mark as completed to avoid re-processing
			}
			
			goto waitForCompletion
		case guard <- struct{}{}:
			wg.Add(1)
			assetWg.Add(1)
			archivedCount++

			if cfg.Headless {
				go func(item *models.Item) {
					defer assetWg.Done()
					headless.ArchiveItem(item, wg, guard, bucketManager, client)
				}(item)
			} else {
				go func(item *models.Item) {
					defer assetWg.Done()
					general.ArchiveItem(item, wg, guard, bucketManager, client)
				}(item)
			}
		}
	}

waitForCompletion:
	// Wait for all started asset archiving to complete
	assetWg.Wait()
	
	if skippedCount > 0 {
		logger.Info("assets archiving completed with timeout", "archived", archivedCount, "skipped", skippedCount, "timeout", cfg.AssetsArchivingTimeout)
	} else {
		logger.Debug("assets archiving completed within timeout", "archived", archivedCount, "timeout", cfg.AssetsArchivingTimeout)
	}
}
