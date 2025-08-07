package general

import (
	"context"
	"io"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/archiver/body"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/discard/reasoncode"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/ratelimiter"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/domainscrawl"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/internetarchive/Zeno/pkg/models"
	warc "github.com/internetarchive/gowarc"
)

func ArchiveItem(item *models.Item, wg *sync.WaitGroup, guard chan struct{}, globalBucketManager *ratelimiter.BucketManager, client *warc.CustomHTTPClient) {
	defer wg.Done()
	defer func() { <-guard }()
	defer stats.URLsCrawledIncr()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "archiver.general.archive",
		"item_id":   item.GetShortID(),
		"seed_id":   item.GetSeed().GetShortID(),
		"depth":     item.GetDepth(),
		"hops":      item.GetURL().GetHops(),
		"url":       item.GetURL().String(),
	})

	var (
		err             error
		resp            *http.Response
		feedbackChan    chan struct{}
		wrappedConnChan chan *warc.CustomConnection
		conn            *warc.CustomConnection
	)

	// Execute the request
	req := item.GetURL().GetRequest()
	if req == nil {
		panic("request is nil")
	}

	// Wait for the rate limiter if enabled
	if globalBucketManager != nil {
		elapsed := globalBucketManager.Wait(req.URL.Host)
		logger.Debug("got token from bucket", "elapsed", elapsed)
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

		resp, err = client.Do(req)
		if err != nil {
			if retry < config.Get().MaxRetry {
				logger.Warn("retrying request", "err", err.Error(), "retry", retry, "sleep_time", retrySleepTime)
				time.Sleep(retrySleepTime)
				continue
			}

			// retries exhausted
			logger.Error("unable to execute request", "err", err.Error())
			item.SetStatus(models.ItemFailed)
			return
		}
		conn = <-wrappedConnChan

		discarded := false
		discardReason := ""
		if client.DiscardHook == nil {
			discardReason = reasoncode.HookNotSet
		} else {
			discarded, discardReason = client.DiscardHook(resp)
		}
		isBadStatusCode := resp.StatusCode >= 500 || slices.Contains([]int{408, 425, 429}, resp.StatusCode)

		if discarded {
			resp.Body.Close()              // First, close the body, to stop downloading data anymore.
			io.Copy(io.Discard, resp.Body) // Then, consume the buffer.
		} else if isBadStatusCode {
			// Consume and close the body before retrying
			copyErr := body.CloseConnWithError(logger, conn, body.CopyWithTimeout(io.Discard, resp.Body))
			if copyErr != nil {
				logger.Warn("copyWithTimeout failed for bad status code response", "err", copyErr.Error())
			}
			resp.Body.Close()
		}

		// Retries on:
		// 	- 5XX, 408, 425 and 429
		// 	- Discarded challenge pages (Cloudflare, Akamai, etc.)
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
				logger.Warn("retrying", "reason", retryReason, "retry", retry, "sleep_time", retrySleepTime, "status_code", resp.StatusCode, "url", req.URL)
				time.Sleep(retrySleepTime)
				continue
			} else {
				logger.Error("retries exceeded", "reason", retryReason, "status_code", resp.StatusCode, "url", req.URL)
				item.SetStatus(models.ItemFailed)
				return
			}
		}

		// Discarded
		if discarded {
			logger.Warn("response was blocked by DiscardHook", "reason", discardReason, "status_code", resp.StatusCode)
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

	resp.Body = &body.BodyWithConn{ // Wrap the response body to hold the connection
		ReadCloser: resp.Body,
		Conn:       <-wrappedConnChan,
	}

	// Set the response in the URL
	item.GetURL().SetResponse(resp)

	// Process the body and measure the time
	processStartTime := time.Now()
	err = ProcessBody(item.GetURL(), config.Get().DisableAssetsCapture, domainscrawl.Enabled(), config.Get().MaxHops, config.Get().WARCTempDir, logger)
	if err != nil {
		logger.Error("unable to process body", "err", err.Error())
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

	logger.Info("url archived", "status", resp.StatusCode)

	item.SetStatus(models.ItemArchived)
}
