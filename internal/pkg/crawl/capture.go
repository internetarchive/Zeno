package crawl

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
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

	trace := &httptrace.ClientTrace{
		DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
			c.URLsPerSecond.Incr(1)
			if dnsInfo.Err == nil {
				log.WithFields(log.Fields{
					"crawled":        c.Crawled.Value(),
					"host":           item.Host,
					"rate":           c.URLsPerSecond.Rate(),
					"source_url":     item.URL.String(),
					"active_workers": c.ActiveWorkers.Value(),
					"hop":            item.Hop,
				}).Debug("dns:" + item.Host)
			}
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	if _, err := http.DefaultTransport.RoundTrip(req); err != nil {
		return outlinks, err
	}

	// Execute GET request
	resp, err := c.Client.Do(req)
	if err != nil {
		return outlinks, err
	}

	log.WithFields(log.Fields{
		"queued":         c.Frontier.QueueCount.Value(),
		"crawled":        c.Crawled.Value(),
		"host":           item.Host,
		"rate":           c.URLsPerSecond.Rate(),
		"status_code":    resp.StatusCode,
		"active_workers": c.ActiveWorkers.Value(),
		"hop":            item.Hop,
	}).Info(item.URL.String())

	// Read body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		return outlinks, err
	}

	// Extract outlinks
	outlinks = extractOutlinksRegex(string(body))

	resp.Body.Close()
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
	if needBrowser(item) && c.Headless == true {
		outlinks, err = c.captureWithBrowser(ctx, item)
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
