package crawl

import (
	"net/http"
	"net/url"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/go-rod/rod"
	"github.com/go-rod/stealth"
	"github.com/sirupsen/logrus"
)

func (c *Crawl) captureHeadless(item *frontier.Item) (respBody string, respHeaders http.Header, err error) {
	var (
		capturedAssets []url.URL
		executionStart = time.Now()
	)

	// Set the hijack router
	router := c.HeadlessBrowser.HijackRequests()
	defer router.MustStop()

	router.MustAdd("*", func(ctx *rod.Hijack) {
		startAssetCapture := time.Now()

		logrus.Infof("Capturing asset: %s", ctx.Request.URL().String())

		// If the response is for the main page, save the body
		if ctx.Request.URL().String() == item.URL.String() {
			_ = ctx.LoadResponse(&c.Client.Client, true)

			respBody = ctx.Response.Body()
			respHeaders = ctx.Response.Headers().Clone()
		} else {
			// Cases for which we do not want to load the request:
			// - If the URL is in the list of captured assets
			// - If the URL is in the list of excluded hosts
			// - If the URL is in the seencheck database
			if utils.URLInSlice(ctx.Request.URL(), capturedAssets) {
				return
			} else if utils.StringInSlice(ctx.Request.URL().Host, c.ExcludedHosts) {
				return
			} else if c.Seencheck {
				seen := c.seencheckURL(utils.URLToString(ctx.Request.URL()), "asset")
				if seen {
					return
				}
			} else if c.UseHQ {
				new, err := c.HQSeencheckURL(ctx.Request.URL())
				if err != nil {
					logWarning.WithFields(logrus.Fields{
						"error": err,
					}).Error("Unable to check if URL is in HQ seencheck database")
				}

				if !new {
					return
				}
			}

			// Load the response
			_ = ctx.LoadResponse(&c.Client.Client, true)

			var assetItem = &frontier.Item{
				URL:        ctx.Request.URL(),
				ParentItem: item,
				Type:       "asset",
			}

			capturedAssets = append(capturedAssets, *ctx.Request.URL())

			c.logCrawlSuccess(startAssetCapture, -1, assetItem)
		}
	})

	go router.Run()

	// Create a new page
	page := stealth.MustPage(c.HeadlessBrowser)
	defer page.MustClose()

	// Navigate to the URL
	err = page.Timeout(c.Client.Timeout).Navigate(utils.URLToString(item.URL))
	if err != nil {
		return
	}

	// Wait for the page to load
	err = page.Timeout(c.Client.Timeout).WaitLoad()
	if err != nil {
		return
	}

	// If --headless-wait-after-load is enabled, wait for the specified duration
	if c.HeadlessWaitAfterLoad > 0 {
		time.Sleep(time.Duration(c.HeadlessWaitAfterLoad) * time.Second)
	}

	c.logCrawlSuccess(executionStart, -1, item)

	return respBody, respHeaders, nil
}
