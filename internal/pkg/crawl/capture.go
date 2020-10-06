package crawl

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/warc"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"
)

func (c *Crawl) captureWithBrowser(ctx context.Context, item *frontier.Item) (outlinks []url.URL, err error) {
	// Log requests
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *network.EventResponseReceived:
			if strings.Compare(ev.Response.URL, item.URL.String()) == 0 {
				log.WithFields(log.Fields{
					"status_code":    ev.Response.Status,
					"active_workers": c.ActiveWorkers.Value(),
					"hop":            item.Hop,
				}).Info(ev.Response.URL)
				c.URLsPerSecond.Incr(1)
				c.Crawled.Incr(1)
			} else {
				log.WithFields(log.Fields{
					"type":           "asset",
					"status_code":    ev.Response.Status,
					"active_workers": c.ActiveWorkers.Value(),
					"hop":            item.Hop,
				}).Debug(ev.Response.URL)
				c.URLsPerSecond.Incr(1)
				c.Crawled.Incr(1)
			}
		}
	})

	// Run task
	err = chromedp.Run(ctx,
		network.Enable(),
		chromedp.Navigate(item.URL.String()),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if c.MaxHops > 0 {
				// Extract outer HTML
				node, err := dom.GetDocument().Do(ctx)
				if err != nil {
					return err
				}
				str, err := dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)
				if err != nil {
					return err
				}

				// Extract outlinks
				outlinks = extractOutlinksRegex(str)

				return err
			}

			return err
		}),
	)
	if err != nil {
		return outlinks, err
	}

	return outlinks, nil
}

func (c *Crawl) captureWithGET(ctx context.Context, item *frontier.Item) (outlinks []url.URL, err error) {
	// Prepare GET request
	req, err := http.NewRequest("GET", item.URL.String(), nil)
	if err != nil {
		return outlinks, err
	}

	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept-Encoding", "*/*")
	if item.Hop > 0 {
		req.Header.Set("Referer", item.ParentItem.URL.String())
	} else {
		req.Header.Set("Referer", item.URL.String())
	}

	// This retry loop is used for when the WARC writing fail,
	// it can happen when the response have an issue, such as
	// an unexpected EOF. That happens often when using proxies.
	for retryCount := 1; retryCount <= c.WARCRetry; retryCount++ {
		// Execute GET request
		resp, err := c.Client.Do(req)
		if err != nil {
			return outlinks, err
		}
		defer resp.Body.Close()

		// Write response and request
		var warcWritingTime time.Duration
		if c.WARC {
			records, err := warc.RecordsFromHTTPResponse(resp)
			if err != nil {
				log.WithFields(log.Fields{
					"url":   req.URL.String(),
					"error": err,
				}).Error("error when turning HTTP resp into WARC records, retrying.. ", retryCount, "/", c.WARCRetry)
				resp.Body.Close()

				// If the crawl is finishing, we do not want to keep
				// retrying the requests, instead we just want to finish
				// all workers execution.
				if c.Finished.Get() {
					outlinks = append(outlinks, *req.URL)
					return outlinks, err
				}

				continue
			} else {
				startPush := time.Now()
				c.WARCWriter <- records
				warcWritingTime = time.Since(startPush)
			}
		}

		log.WithFields(log.Fields{
			"queued":            c.Frontier.QueueCount.Value(),
			"crawled":           c.Crawled.Value(),
			"rate":              c.URLsPerSecond.Rate(),
			"status_code":       resp.StatusCode,
			"active_workers":    c.ActiveWorkers.Value(),
			"hop":               item.Hop,
			"warc_writing_time": warcWritingTime,
		}).Info(item.URL.String())

		// Extract assets and outlinks
		if item.Hop < c.MaxHops {
			outlinks, err := extractOutlinksGoquery(resp)
			if err != nil {
				return outlinks, err
			}
			return outlinks, nil
		}
		return outlinks, nil
	}
	return outlinks, nil
}

// Capture capture a page and queue the outlinks
func (c *Crawl) capture(item *frontier.Item) (outlinks []url.URL, err error) {
	var ctx context.Context

	// Create context for headless requests
	if c.Headless {
		ctx, cancel := chromedp.NewContext(context.Background())
		// Useless but avoir warning
		_ = ctx
		defer cancel()
	}

	// Check with HTTP HEAD request if the URL need a full headless browser or a simple GET request
	if c.Headless {
		if needBrowser(item) {
			c.URLsPerSecond.Incr(1)
			outlinks, err = c.captureWithGET(ctx, item)
			c.Crawled.Incr(1)
		} else {
			outlinks, err = c.captureWithBrowser(ctx, item)
		}
	} else {
		c.URLsPerSecond.Incr(1)
		outlinks, err = c.captureWithGET(ctx, item)
		c.Crawled.Incr(1)
	}

	if err != nil {
		return nil, err
	}

	return outlinks, nil
}
