package crawl

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
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
			err = ctx.LoadResponse(&c.Client.Client, true)
			if err != nil {
				c.Logger.Error(err)
			}

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

	// Set some cookies
	page, err = setHeadlessCookies(page, item)
	if err != nil {
		return
	}

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

	// Scroll to the bottom of the page
	js := `
		() =>{ 
			(function() {
				var intervalObj = null;
				var retry = 0;
				var clickHandler = function() { 
					console.log("Clicked; stopping autoscroll");
					clearInterval(intervalObj);
					document.body.removeEventListener("click", clickHandler);
				}
				function scrollDown() { 
					var scrollHeight = document.body.scrollHeight,
						scrollTop = document.body.scrollTop,
						innerHeight = window.innerHeight,
						difference = (scrollHeight - scrollTop) - innerHeight
			
					if (difference > 0) { 
						window.scrollBy(0, difference);
						if (retry > 0) { 
							retry = 0;
						}
						console.log("scrolling down more");
					} else {
						if (retry >= 3) {
							console.log("reached bottom of page; stopping");
							clearInterval(intervalObj);
							document.body.removeEventListener("click", clickHandler);
						} else {
							console.log("[apparenty] hit bottom of page; retrying: " + (retry + 1));
							retry++;
						}
					}
				}
			
				document.body.addEventListener("click", clickHandler);
				intervalObj = setInterval(scrollDown, 1000);
			})()
		}
	`

	_, err = page.Timeout(c.Client.Timeout).Eval(js)
	if err != nil {
		logrus.Fatalf("unable to scroll to the bottom of the page: %s", err)
		return
	}

	// Wait for the page to be stable
	err = page.Timeout(c.Client.Timeout).WaitStable(time.Second * 2)
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

func setHeadlessCookies(page *rod.Page, item *frontier.Item) (newPage *rod.Page, err error) {
	var cookies []*proto.NetworkCookieParam

	if strings.Contains(item.Host, "youtube") {
		cookies = append(cookies, &proto.NetworkCookieParam{
			Name:   "SOCS",
			Value:  "CAESEwgDEgk0ODE3Nzk3MjQaAmVuIAEaBgiA_LyaBg",
			Domain: ".youtube.com",
			Path:   "/",
		})
	}

	if len(cookies) > 0 {
		err := page.SetCookies(cookies)
		if err != nil {
			return nil, err
		}
	}

	return page, nil
}
