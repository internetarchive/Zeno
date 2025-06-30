package archiver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gabriel-vasile/mimetype"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/discard/reasoncode"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/ratelimiter"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/domainscrawl"
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
	log.Start()
	logger = log.NewFieldedLogger(&log.Fields{
		"component": "archiver",
	})

	stats.Init()

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

	for i := range items {
		if items[i].GetStatus() != models.ItemPreProcessed {
			logger.Debug("skipping item", "seed_id", seed.GetShortID(), "item_id", items[i].GetShortID(), "status", items[i].GetStatus(), "depth", items[i].GetDepth())
			continue
		}

		guard <- struct{}{}

		wg.Add(1)
		go func(item *models.Item) {
			defer wg.Done()
			defer func() { <-guard }()
			defer stats.URLsCrawledIncr()

			var (
				err             error
				resp            *http.Response
				feedbackChan    chan struct{}
				wrappedConnChan chan *warc.CustomConnection
			)

			// Execute the request
			req := item.GetURL().GetRequest()
			if req == nil {
				panic("request is nil")
			}

			// Wait for the rate limiter if enabled
			if globalBucketManager != nil {
				elapsed := globalBucketManager.Wait(req.URL.Host)
				logger.Debug("got token from bucket", "seed_id", seed.GetShortID(), "item_id", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops(), "elapsed", elapsed)
			}

			// Don't use the global bucket manager in the retry loop.
			// Most failed requests won't reach the server anyway, so we don't need to wait for the rate limit.
			// This prevents workers from being blocked for too long by dead sites, such as host unreachable or DNS errors.
			for retry := 0; retry <= config.Get().MaxRetry; retry++ {
				// This is unused unless there is an error
				retrySleepTime := time.Second * time.Duration(retry*2)

				// Get and measure request time
				getStartTime := time.Now()

				// If WARC writing is asynchronous, we don't need a feedback channel
				if !config.Get().WARCWriteAsync {
					feedbackChan = make(chan struct{}, 1)
					// Add the feedback channel to the request context
					req = req.WithContext(context.WithValue(req.Context(), "feedback", feedbackChan))
				}
				wrappedConnChan = make(chan *warc.CustomConnection, 1)
				req = req.WithContext(context.WithValue(req.Context(), "wrappedConn", wrappedConnChan))

				var client *warc.CustomHTTPClient
				if config.Get().Proxy != "" {
					client = globalArchiver.ClientWithProxy
				} else {
					client = globalArchiver.Client
				}

				resp, err = client.Do(req)
				if err != nil {
					if retry < config.Get().MaxRetry {
						logger.Warn("retrying request", "err", err.Error(), "seed_id", seed.GetShortID(), "item_id", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops(), "retry", retry, "sleep_time", retrySleepTime)
						time.Sleep(retrySleepTime)
						continue
					}

					// retries exhausted
					logger.Error("unable to execute request", "err", err.Error(), "seed_id", seed.GetShortID(), "item_id", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops())
					item.SetStatus(models.ItemFailed)
					return
				}

				discarded := false
				discardReason := ""
				if client.DiscardHook == nil {
					discardReason = reasoncode.HookNotSet
				} else {
					discarded, discardReason = client.DiscardHook(resp)
				}

				if discarded {
					// Consume body, needed to avoid leaking RAM & storage
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}

				// Retries on:
				// 	- 5XX, 408, 425 and 429
				// 	- Discarded challenge pages (Cloudflare, Akamai, etc.)
				isBadStatusCode := resp.StatusCode >= 500 || slices.Contains([]int{408, 425, 429}, resp.StatusCode)
				isDiscardedChallengePage := discarded && reasoncode.IsChallengePage(discardReason)
				if isBadStatusCode || isDiscardedChallengePage {
					if globalBucketManager != nil {
						globalBucketManager.AdjustOnFailure(req.URL.Host, resp.StatusCode)
					}

					retryReason := "bad response code"
					if isDiscardedChallengePage {
						retryReason = discardReason
					}

					if retry < config.Get().MaxRetry {
						logger.Warn("retrying", "reason", retryReason, "seed_id", seed.GetShortID(), "item_id", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops(), "retry", retry, "sleep_time", retrySleepTime, "status_code", resp.StatusCode, "url", req.URL)
						time.Sleep(retrySleepTime)
						continue
					} else {
						logger.Error("retries exceeded", "reason", retryReason, "seed_id", seed.GetShortID(), "item_id", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops(), "status_code", resp.StatusCode, "url", req.URL)
						item.SetStatus(models.ItemFailed)
						return
					}
				}

				// Discarded
				if discarded {
					logger.Warn("response was blocked by DiscardHook", "reason", discardReason, "seed_id", seed.GetShortID(), "item_id", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops(), "status_code", resp.StatusCode, "url", req.URL)
					item.SetStatus(models.ItemFailed)
					return
				}

				// OK
				if globalBucketManager != nil {
					globalBucketManager.OnSuccess(req.URL.Host)
				}

				stats.MeanHTTPRespTimeAdd(time.Since(getStartTime))
				break
			}

			conn := <-wrappedConnChan
			resp.Body = &BodyWithConn{ // Wrap the response body to hold the connection
				ReadCloser: resp.Body,
				Conn:       conn,
			}

			// Set the response in the URL
			item.GetURL().SetResponse(resp)

			// Process the body and measure the time
			processStartTime := time.Now()
			err = ProcessBody(item.GetURL(), config.Get().DisableAssetsCapture, domainscrawl.Enabled(), config.Get().MaxHops, config.Get().WARCTempDir)
			if err != nil {
				logger.Error("unable to process body", "err", err.Error(), "item_id", item.GetShortID(), "seed_id", seed.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops())
				item.SetStatus(models.ItemFailed)
				return
			}

			stats.MeanProcessBodyTimeAdd(time.Since(processStartTime))
			stats.HTTPReturnCodesIncr(strconv.Itoa(resp.StatusCode))

			// If WARC writing is asynchronous, we don't need to wait for the feedback channel
			if !config.Get().WARCWriteAsync {
				feedbackTime := time.Now()
				// Waiting for WARC writing to finish
				<-feedbackChan
				stats.MeanWaitOnFeedbackTimeAdd(time.Since(feedbackTime))
			}

			logger.Info("url archived", "url", item.GetURL(), "seed_id", seed.GetShortID(), "item_id", item.GetShortID(), "depth", item.GetDepth(), "hops", item.GetURL().GetHops(), "status", resp.StatusCode)

			item.SetStatus(models.ItemArchived)
		}(items[i])
	}

	// Wait for all goroutines to finish
	wg.Wait()
}
