package crawl

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/warc"
	"github.com/sirupsen/logrus"
	"github.com/zeebo/xxh3"
)

func (c *Crawl) captureAsset(URL *url.URL, parent *frontier.Item) error {
	var executionStart = time.Now()

	// Prepare GET request
	req, err := http.NewRequest("GET", URL.String(), nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept-Encoding", "*/*")
	req.Header.Set("Referer", parent.URL.String())

	// This retry loop is used for when the WARC writing fail,
	// it can happen when the response have an issue, such as
	// an unexpected EOF. That happens often when using proxies.
	for retryCount := 1; retryCount <= c.WARCRetry; retryCount++ {
		// Execute GET request
		c.URLsPerSecond.Incr(1)
		resp, err := c.Client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Write response and reques
		if c.WARC {
			records, err := warc.RecordsFromHTTPResponse(resp)
			if err != nil {
				log.WithFields(logrus.Fields{
					"url":   req.URL.String(),
					"type":  "asset",
					"error": err,
				}).Error("error when turning HTTP resp into WARC records, retrying.. ", retryCount, "/", c.WARCRetry)
				resp.Body.Close()

				// If the crawl is finishing, we do not want to keep
				// retrying the requests, instead we just want to finish
				// all workers execution.
				if c.Finished.Get() {
					return nil
				}

				continue
			} else {
				c.WARCWriter <- records
			}
		}

		c.Crawled.Incr(1)
		log.WithFields(logrus.Fields{
			"queued":         c.Frontier.QueueCount.Value(),
			"crawled":        c.Crawled.Value(),
			"rate":           c.URLsPerSecond.Rate(),
			"status_code":    resp.StatusCode,
			"active_workers": c.ActiveWorkers.Value(),
			"hop":            parent.Hop,
			"type":           "asset",
			"execution_time": time.Since(executionStart),
		}).Info(URL)

		return nil
	}
	return nil
}

func (c *Crawl) captureWithGET(item *frontier.Item) (outlinks []url.URL, err error) {
	var executionStart = time.Now()

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
		if c.WARC {
			records, err := warc.RecordsFromHTTPResponse(resp)
			if err != nil {
				log.WithFields(logrus.Fields{
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
				c.WARCWriter <- records
			}
		}

		log.WithFields(logrus.Fields{
			"queued":         c.Frontier.QueueCount.Value(),
			"crawled":        c.Crawled.Value(),
			"rate":           c.URLsPerSecond.Rate(),
			"status_code":    resp.StatusCode,
			"active_workers": c.ActiveWorkers.Value(),
			"hop":            item.Hop,
			"type":           "seed",
			"execution_time": time.Since(executionStart),
		}).Info(item.URL.String())

		// Extract and capture assets
		assets, doc, err := extractAssets(resp)
		if err != nil {
			return outlinks, err
		}

		c.Frontier.QueueCount.Incr(int64(len(assets)))
		for _, asset := range assets {
			c.Frontier.QueueCount.Incr(-1)

			// Just making sure we do not over archive
			if item.URL.String() == asset.String() {
				continue
			}

			// If --seencheck is enabled, then we check if the URI is in the
			// seencheck DB before doing anything. If it is in it, we skip the item
			if c.Frontier.UseSeencheck {
				hash := strconv.FormatUint(xxh3.HashString(asset.String()), 10)
				if c.Frontier.Seencheck.IsSeen(hash) {
					continue
				}
				c.Frontier.Seencheck.Seen(hash)
			}

			err = c.captureAsset(&asset, item)
			if err != nil {
				log.WithFields(logrus.Fields{
					"error":          err,
					"queued":         c.Frontier.QueueCount.Value(),
					"crawled":        c.Crawled.Value(),
					"rate":           c.URLsPerSecond.Rate(),
					"active_workers": c.ActiveWorkers.Value(),
					"parent_hop":     item.Hop,
					"parent_url":     item.URL.String(),
					"type":           "asset",
				}).Warning(asset.String())
				continue
			}
		}

		// Extract outlinks
		if item.Hop < c.MaxHops {
			outlinks, err := extractOutlinks(resp, doc)
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
	// Check with HTTP HEAD request if the URL need a full headless browser or a simple GET request
	c.URLsPerSecond.Incr(1)
	outlinks, err = c.captureWithGET(item)
	c.Crawled.Incr(1)
	if err != nil {
		return nil, err
	}

	return outlinks, nil
}
