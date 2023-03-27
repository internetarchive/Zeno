package crawl

import (
	"net/http"
	"net/url"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/go-rod/rod"
	"github.com/go-rod/stealth"
)

func (c *Crawl) captureHeadless(item *frontier.Item) (respBody string, respHeaders http.Header, capturedAssets []*url.URL, err error) {
	// Set the hijack router
	router := c.HeadlessBrowser.HijackRequests()
	defer router.MustStop()

	router.MustAdd("*", func(ctx *rod.Hijack) {
		// TODO: add some headers like Referer & custom User-Agent
		// ctx.Request.Req().Header.Set("My-Header", "test")

		// LoadResponse runs the default request to the destination of the request.
		// Not calling this will require you to mock the entire response.
		// This can be done with the SetXxx (Status, Header, Body) functions on the
		// ctx.Response struct.
		_ = ctx.LoadResponse(&c.Client.Client, true)

		// If the response is for the main page, save the body
		if ctx.Request.URL().String() == item.URL.String() {
			respBody = ctx.Response.Body()
			respHeaders = ctx.Response.Headers().Clone()
		}

		// Add the asset to the list of captured assets if it's not already in it
		// TODO: do we want to skip that if LoadResponse failed?
		if !utils.URLInSlice(ctx.Request.URL(), capturedAssets) {
			capturedAssets = append(capturedAssets, ctx.Request.URL())
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
	err = page.WaitLoad()
	if err != nil {
		return
	}

	return respBody, respHeaders, capturedAssets, nil
}
