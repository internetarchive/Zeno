package headless

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/body"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/discard/reasoncode"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/ratelimiter"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor"
	"github.com/internetarchive/Zeno/pkg/models"
	warc "github.com/internetarchive/gowarc"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

var archiverLogger = log.NewFieldedLogger(&log.Fields{"component": "archiver.headless.archiver"})

//go:embed behaviors.js
var behaviorsJS string

func behaviorInitJS() string {
	options := map[string]any{
		"autoscroll":   false,
		"autofetch":    false, // disabled by default, this function will fetch resources twice.
		"autoplay":     false,
		"autoclick":    false, // disabled by default, the popup babble will not be closed automatically
		"siteSpecific": false,

		"timeout": config.Get().HeadlessBehaviorTimeout.Milliseconds(),
		"log":     "__zeno_bx_log",
	}

	// Enable behaviors based on the configuration
	for _, b := range config.Get().HeadlessBehaviors {
		switch b {
		case "autofetch":
			logger.Warn("autofetch behavior is enabled, this will cause the browser to fetch resources TWICE")
		case "autoclick":
			logger.Warn("autoclick behavior is enabled, this behavior probably will not work as expected (?), be careful")
		}
		options[b] = true
	}

	var parts []string
	for k, v := range options {
		parts = append(parts, fmt.Sprintf("%s: %v", k, v))
	}

	return fmt.Sprintf("self.__bx_behaviors.init({%s});", strings.Join(parts, ", "))
}

var behaviorRunJS = `async u => {
	await self.__bx_behaviors.run();
}`

func clientDo(client *http.Client, req *http.Request, h *rod.Hijack) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// TODO: handle hijack response in another function
	h.Response.Payload().ResponseCode = resp.StatusCode
	h.Response.RawResponse = resp

	for k, vs := range resp.Header {
		for _, v := range vs {
			h.Response.SetHeader(k, v)
		}
	}

	return resp, nil
}

func ArchiveItem(item *models.Item, wg *sync.WaitGroup, guard chan struct{}, bucketManager *ratelimiter.BucketManager, client *warc.CustomHTTPClient) {
	defer wg.Done()
	defer func() { <-guard }()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "archiver.headless.archive.item",
		"item_id":   item.GetShortID(),
		"item_url":  item.GetURL().String(),
	})

	err := archivePage(client, item, item.GetSeed(), bucketManager)
	if err != nil {
		item.SetStatus(models.ItemFailed)
		logger.Error("unable to archive page in headless mode", "err", err.Error())
		return
	}

	// If headless mode is enabled, we don't need to process the body
	item.SetStatus(models.ItemArchived)
	logger.Info("page archived successfully")
}

func archivePage(warcClient *warc.CustomHTTPClient, item *models.Item, seed *models.Item, bucketManager *ratelimiter.BucketManager) error {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "archiver.headless.archive.page",
		"item_id":   item.GetShortID(),
		"seed_id":   seed.GetShortID(),
		"item_url":  item.GetURL().String(),
	})
	seenRequests := make([]string, 0)
	defer seencheck(item, seed, &seenRequests)
	bxLogger := newBxLogger(item)

	var err error

	// Set the hijack router
	router := HeadlessBrowser.HijackRequests()
	defer router.MustStop()

	flyingRequests := NewWaitGroup()

	requestsMutex := &sync.Mutex{}
	router.MustAdd("*", func(hijack *rod.Hijack) {
		logger := log.NewFieldedLogger(&log.Fields{
			"component": "archiver.headless.router",
			"item_id":   item.GetShortID(),
			"seed_id":   seed.GetShortID(),
			"item_url":  item.GetURL().String(),
			"url":       hijack.Request.URL().String(),
		})

		requestsMutex.Lock()
		seenRequests = append(seenRequests, hijack.Request.URL().String())
		requestsMutex.Unlock()

		flyingRequests.Add(1, hijack.Request.URL().String())
		defer flyingRequests.Done(hijack.Request.URL().String())

		// drop requests that are not in the allowed methods
		if !slices.Contains(config.Get().HeadlessAllowedMethods, hijack.Request.Method()) {
			logger.Debug("dropping request not in allowed methods", "method", hijack.Request.Method())
			hijack.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
			return
		}

		var (
			req             *http.Request
			resp            *http.Response
			feedbackChan    chan struct{}
			wrappedConnChan chan *warc.CustomConnection
		)

		if hijack.Request.URL().String() == item.GetURL().String() {
			logger.Debug("capturing main page")
		} else {
			logger.Debug("capturing asset")
		}

		if bucketManager != nil {
			bucketManager.Wait(hijack.Request.URL().Host)
		}
		req = hijack.Request.Req()

		for retry := 0; retry <= config.Get().MaxRetry; retry++ {
			// This is unused unless there is an error
			retrySleepTime := time.Second * time.Duration(retry*2)

			// // Get and measure request time
			// getStartTime := time.Now()

			// If WARC writing is asynchronous, we don't need a feedback channel
			if !config.Get().WARCWriteAsync {
				feedbackChan = make(chan struct{}, 1)
				// Add the feedback channel to the request context
				req = req.WithContext(context.WithValue(req.Context(), "feedback", feedbackChan))
			}
			// Prepare warppedConn channel
			wrappedConnChan = make(chan *warc.CustomConnection, 1)
			req = req.WithContext(context.WithValue(req.Context(), "wrappedConn", wrappedConnChan))

			// Set UA if not in stealth mode
			if !config.Get().HeadlessStealth {
				req.Header.Set("User-Agent", config.Get().UserAgent)
			}

			// If the response is for the main page, save the body
			resp, err = clientDo(&warcClient.Client, req, hijack)
			if err != nil {
				if errors.Is(err, context.Canceled) { // failfast if the request is canceled
					logger.Debug("request canceled", "err", err.Error())
					hijack.Response.Fail(proto.NetworkErrorReasonTimedOut)
					return
				}
				if retry < config.Get().MaxRetry {
					logger.Warn("retrying request", "err", err.Error(), "retry", retry, "sleep_time", retrySleepTime)
					time.Sleep(retrySleepTime)
					continue
				}

				// retries exhausted
				logger.Error("unable to execute request", "err", err.Error())
				hijack.Response.Fail(proto.NetworkErrorReasonAborted)
				return
			}
			break
		}

		if bucketManager != nil {
			bucketManager.OnSuccess(hijack.Request.URL().Host)
		}

		discarded := false
		discardReason := ""
		if warcClient.DiscardHook == nil {
			discardReason = reasoncode.HookNotSet
		} else {
			discarded, discardReason = warcClient.DiscardHook(resp)
		}

		if discarded {
			resp.Body.Close()              // First, close the body, to stop downloading data anymore.
			io.Copy(io.Discard, resp.Body) // Then, consume the buffer.

			logger.Warn("response was blocked by DiscardHook", "reason", discardReason, "status_code", resp.StatusCode)
			hijack.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
			return
		}

		resp.Body = &body.BodyWithConn{ // Wrap the response body to hold the connection
			ReadCloser: resp.Body,
			Conn:       <-wrappedConnChan,
		}

		fullBody, err := ProcessBodyHeadless(hijack, resp)
		if err != nil {
			logger.Error("unable to process body", "error", err)
			hijack.Response.Fail(proto.NetworkErrorReasonConnectionFailed)
			return
		}

		// OK

		if len(fullBody) == 0 { // ([]uint8) <nil>
			// If the response body is empty (e.g., 30X redirects), We have to set it to an empty byte slice
			// so that the Rod knows that the response payload is valid empty.
			// Else, The browser will wait for the body to be filled and will never finish the request.
			fullBody = []byte{} // ([]uint8) {}
		}
		hijack.Response.Payload().Body = fullBody
		fullBody = nil

		logger.Debug("processed body", "size", len(hijack.Response.Payload().Body))
	})

	go router.Run()

	// Create a new page
	logger.Debug("creating new page for browser")
	var page *rod.Page
	if config.Get().HeadlessStealth {
		logger.Debug("using stealth for browser")
		page = stealth.MustPage(HeadlessBrowser)
	} else {
		page = HeadlessBrowser.MustPage()
	}
	defer page.MustClose()

	logger.Debug("Injecting behaviors.js...")
	page.MustEvalOnNewDocument(behaviorsJS)

	page.Expose("__zeno_bx_log", bxLogger.LogFunc)

	logger.Debug("using page behaviors", "initJS", behaviorInitJS())
	page.MustEvalOnNewDocument(behaviorInitJS())

	// TODO: Set cookies if needed (if no other cookies for this URL are set)

	// Navigate to the URL
	logger.Debug("navigating to URL")
	err = page.Navigate(item.GetURL().String())
	if err != nil {
		logger.Error("unable to navigate to URL", "error", err)
		return err
	}

	// Wait for the page to load
	logger.Info("waiting for page to load", "timeout", config.Get().HeadlessPageLoadTimeout)
	err = page.Timeout(config.Get().HeadlessPageLoadTimeout).WaitLoad()
	if err != nil {
		logger.Warn("unable to wait for page to load", "error", err)
	}

	info, err := page.Info()
	if err != nil {
		logger.Debug("unable to get page info", "error", err)
	} else {
		logger.Debug("page info", "title", info.Title)
	}

	// if --post-load-delay is set, wait for the specified delay
	if config.Get().HeadlessPostLoadDelay > 0 {
		logger.Debug("waiting for post-load delay", "delay", config.Get().HeadlessPostLoadDelay)
		time.Sleep(config.Get().HeadlessPostLoadDelay)
	}

	// Run the behaviors script
	logger.Debug("running behaviors script", "timeout", config.Get().HeadlessBehaviorTimeout)
	start := time.Now()
	_, err = page.Evaluate(rod.Eval(behaviorRunJS).ByPromise()) // Theg [BehaviorTimeout] is set in the __bx_behaviors.init() call
	if err != nil {
		logger.Error("unable to run behaviors script", "error", err)
	}
	logger.Info("behaviors script done", "elapsed", time.Since(start))

	// Wait for all the inflight requests to finish
	start = time.Now()
	logger.Debug("waiting for all inflight requests to finish")
	flyingRequests.Wait(log.NewFieldedLogger(&log.Fields{
		"component": "archiver.archiveHeadless.wait",
		"seed_id":   seed.GetShortID(),
		"item_id":   item.GetShortID(),
	}), 5*time.Second /* This is progress reporting interval, not the timeout */)
	logger.Debug("all inflight requests finished", "elapsed", time.Since(start))

	warcClient.CloseIdleConnections() // just in case, IDK if this is needed

	item.SetStatus(models.ItemArchived)
	extractAndStoreHTML(item, page)
	return nil
}

// Get the Document from the page and store it in the item
func extractAndStoreHTML(item *models.Item, page *rod.Page) error {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "archiver.headless.archive.extractHTML",
		"item_id":   item.GetShortID(),
		"item_url":  item.GetURL().String(),
	})
	docEl, err := page.Element("*") // get entire document
	if err != nil {
		logger.Error("unable to get document element", "error", err)
		return err
	}

	htmlText, err := docEl.HTML()
	if err != nil {
		logger.Error("unable to convert document element to HTML", "error", err)
		return err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		logger.Error("unable to create goquery document from HTML", "error", err)
		return err
	}

	item.GetURL().SetDocument(doc)

	// Create a temp file with a 8MB memory buffer
	spooledBuff := spooledtempfile.NewSpooledTempFile("zeno", config.Get().WARCTempDir, 8000000, false, -1)
	_, err = io.Copy(spooledBuff, strings.NewReader(htmlText))
	if err != nil {
		closeErr := spooledBuff.Close()
		if closeErr != nil {
			panic(closeErr)
		}
		logger.Error("unable to copy HTML to spooled buffer", "error", err)
	}
	item.GetURL().SetBody(spooledBuff)
	item.GetURL().RewindBody()

	return nil
}

func seencheck(item *models.Item, seed *models.Item, seenRequests *[]string) {
	tmpItem := models.NewItem(&models.URL{Raw: item.GetURL().Raw}, "")
	tmpItem.GetURL().Parse()
	for _, reqURL := range *seenRequests {
		tmpChildItem := models.NewItem(&models.URL{Raw: reqURL}, "")
		tmpChildItem.GetURL().Parse()
		tmpItem.AddChild(tmpChildItem, models.ItemGotChildren)
	}
	if err := preprocessor.GlobalPreprocessor.Seenchecker(tmpItem); err != nil {
		logger.Error("unable to seencheck headless sub-requests", "error", err, "seed_id", seed.GetShortID(), "item_id", item.GetShortID())
	}
}
