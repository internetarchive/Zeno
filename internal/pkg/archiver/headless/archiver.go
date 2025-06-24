package headless

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/pkg/models"
	warc "github.com/internetarchive/gowarc"
)

//go:embed behaviors.js
var behaviorsJS string

func behaviorInitJS() string {
	options := map[string]any{
		"autoscroll":   false,
		"autofetch":    false, // disabled by default, this function will fetch resources twice.
		"autoplay":     false,
		"autoclick":    false, // disabled by default, the popup babble will not be closed automatically
		"siteSpecific": false,

		"timeout": config.Get().BehaviorTimeout.Milliseconds(),
		"log":     "__zeno_log",
	}

	// Enable behaviors based on the configuration
	behaviors := strings.SplitSeq(config.Get().Behaviors, ",")
	for b := range behaviors {
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

func clientDo(h *rod.Hijack, client *http.Client) (*http.Response, error) {
	resp, err := client.Do(h.Request.Req())
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

func ArchiveHeadless(warcClient *warc.CustomHTTPClient, item *models.Item, seed *models.Item) error {
	bxLogger := newBxLogger(item)

	// var resp *http.Response
	var err error

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "archiver.archiveHeadless",
		"item_id":   item.GetShortID(),
		"seed_id":   seed.GetShortID(),
	})

	// Set the hijack router
	router := HeadlessBrowser.HijackRequests()
	defer router.MustStop()

	router.MustAdd("*", func(hijack *rod.Hijack) {
		// drop all non-GET requests
		if hijack.Request.Method() != "GET" {
			logger.Debug("dropping non-GET request", "method", hijack.Request.Method(), "url", hijack.Request.URL().String())
			hijack.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
			return
		}
		var resp *http.Response
		var feedbackChan chan struct{}
		req := hijack.Request.Req()

		// Set UA if not in stealth mode
		if !config.Get().StealthMode {
			req.Header.Set("User-Agent", config.Get().UserAgent)
		}

		// If WARC writing is asynchronous, we don't need a feedback channel
		if !config.Get().WARCWriteAsync {
			feedbackChan = make(chan struct{}, 1)
			// Add the feedback channel to the request context
			req = req.WithContext(context.WithValue(req.Context(), "feedback", feedbackChan))
		}

		defer logger.Debug("asset done", "url", hijack.Request.URL().String())
		// If the response is for the main page, save the body

		if hijack.Request.URL().String() == item.GetURL().String() {
			logger.Debug("capturing main page", "url", hijack.Request.URL().String())
			resp, err = clientDo(hijack, &warcClient.Client)
			if err != nil {
				logger.Error("unable to load response", "error", err)
				hijack.Response.Fail(proto.NetworkErrorReasonConnectionFailed)
				return
			}
		} else {
			logger.Debug("capturing asset", "url", hijack.Request.URL().String())
			resp, err = clientDo(hijack, &warcClient.Client)
			if err != nil {
				logger.Error("unable to load response", "error", err)
				hijack.Response.Fail(proto.NetworkErrorReasonConnectionFailed)
				return
			}
		}

		item.GetURL().SetResponse(resp)

		fullBody, err := ProcessBodyHeadless(hijack, resp)
		if err != nil {
			logger.Error("unable to process body", "error", err, "url", hijack.Request.URL().String())
			hijack.Response.Fail(proto.NetworkErrorReasonConnectionFailed)
			return
		} else {
			if len(fullBody) == 0 { // ([]uint8) <nil>
				// If the response body is empty (e.g., 30X redirects), We have to set it to an empty byte slice
				// so that the Rod knows that the response payload is valid empty.
				// Else, The browser will wait for the body to be filled and will never finish the request.
				fullBody = []byte{} // ([]uint8) {}
			}
			hijack.Response.Payload().Body = fullBody
		}

		logger.Debug("processed body", "url", hijack.Request.URL().String(), "size", len(hijack.Response.Payload().Body))
	})

	go router.Run()

	// Create a new page
	logger.Debug("creating new page for headless browser")
	var page *rod.Page
	if config.Get().StealthMode {
		logger.Debug("using stealth mode for headless browser")
		page = stealth.MustPage(HeadlessBrowser)
	} else {
		page = HeadlessBrowser.MustPage()
	}
	defer page.MustClose()

	logger.Debug("Injecting behaviors.js...", "url", item.GetURL().String())
	page.MustEvalOnNewDocument(behaviorsJS)

	page.Expose("__zeno_log", bxLogger.LogFunc)

	logger.Debug("using page behaviors", "initJS", behaviorInitJS())
	page.MustEvalOnNewDocument(behaviorInitJS())

	// TODO: Set cookies if needed (if no other cookies for this URL are set)

	// Navigate to the URL
	logger.Debug("navigating to URL", "url", item.GetURL().String())
	err = page.Navigate(item.GetURL().String())
	if err != nil {
		logger.Error("unable to navigate to URL", "error", err, "url", item.GetURL().String())
		return err
	}

	// Wait for the page to load
	logger.Info("waiting for page to load", "url", item.GetURL().String(), "timeout", config.Get().PageLoadTimeout)
	err = page.Timeout(config.Get().PageLoadTimeout).WaitLoad()
	if err != nil {
		logger.Warn("unable to wait for page to load", "error", err, "url", item.GetURL().String())
	}

	// if --post-load-delay is set, wait for the specified delay
	if config.Get().PostLoadDelay > 0 {
		logger.Debug("waiting for post-load delay", "delay", config.Get().PostLoadDelay, "url", item.GetURL().String())
		time.Sleep(config.Get().PostLoadDelay)
	}

	// Run the behaviors script
	logger.Debug("running behaviors script", "url", item.GetURL().String(), "timeout", config.Get().BehaviorTimeout)
	start := time.Now()
	_, err = page.Evaluate(rod.Eval(behaviorRunJS).ByPromise()) // The [BehaviorTimeout] is set in the __bx_behaviors.init() call
	if err != nil {
		logger.Error("unable to run behaviors script", "error", err, "url", item.GetURL().String())
	}
	logger.Info("behaviors script done", "elapsed", time.Since(start), "url", item.GetURL().String())

	// Wait for all the ongoing requests to finish
	start = time.Now()
	logger.Debug("waiting for all ongoing requests to finish", "url", item.GetURL().String())
	lifefimeWait := page.Timeout(15 * time.Second).MustWaitRequestIdle()
	lifefimeWait()
	logger.Debug("all ongoing requests finished", "elapsed", time.Since(start), "url", item.GetURL().String())

	warcClient.CloseIdleConnections()
	page.Activate()

	item.SetStatus(models.ItemArchived)
	return nil
}
