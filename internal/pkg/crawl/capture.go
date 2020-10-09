package crawl

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/warc"
	"github.com/sirupsen/logrus"
)

func (c *Crawl) logCrawlSuccess(executionStart time.Time, statusCode int, item *frontier.Item) {
	log.WithFields(logrus.Fields{
		"queued":         c.Frontier.QueueCount.Value(),
		"crawled":        c.Crawled.Value(),
		"rate":           c.URLsPerSecond.Rate(),
		"status_code":    statusCode,
		"active_workers": c.ActiveWorkers.Value(),
		"hop":            item.Hop,
		"type":           item.Type,
		"execution_time": time.Since(executionStart),
	}).Info(item.URL.String())
}

func (c *Crawl) captureAsset(item *frontier.Item) error {
	var executionStart = time.Now()

	// If --seencheck is enabled, then we check if the URI is in the
	// seencheck DB before doing anything. If it is in it, we skip the item
	if c.Frontier.UseSeencheck {
		hash := strconv.FormatUint(item.Hash, 10)
		if c.Frontier.Seencheck.IsSeen(hash) {
			return nil
		}
		c.Frontier.Seencheck.Seen(hash)
	}

	// Prepare GET request
	req, err := http.NewRequest("GET", item.URL.String(), nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept-Encoding", "*/*")
	req.Header.Set("Referer", item.ParentItem.URL.String())

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
		c.Crawled.Incr(1)

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

				// If a redirection is catched, then we execute the redirection
				if resp.StatusCode == 300 || resp.StatusCode == 301 ||
					resp.StatusCode == 302 || resp.StatusCode == 303 ||
					resp.StatusCode == 307 || resp.StatusCode == 308 {

					c.logCrawlSuccess(executionStart, resp.StatusCode, item)

					if resp.Header.Get("location") == item.URL.String() || item.Redirect >= c.MaxRedirect {
						break
					}

					newURL, err := url.Parse(resp.Header.Get("location"))
					if err != nil {
						return err
					}

					newAsset := frontier.NewItem(newURL, item, "asset", item.Hop)
					newAsset.Redirect = item.Redirect + 1
					log.Println("Redirect: ", newAsset.Redirect)
					err = c.captureAsset(newAsset)
					if err != nil {
						return err
					}

					return nil
				}
			}
		}

		c.logCrawlSuccess(executionStart, resp.StatusCode, item)

		return nil
	}
	return nil
}

func (c *Crawl) captureWithGET(item *frontier.Item) (outlinks []url.URL, err error) {
	var executionStart = time.Now()

	// If --seencheck is enabled, then we check if the URI is in the
	// seencheck DB before doing anything. If it is in it, we skip the item
	if c.Frontier.UseSeencheck {
		hash := strconv.FormatUint(item.Hash, 10)
		if c.Frontier.Seencheck.IsSeen(hash) {
			return outlinks, nil
		}
		c.Frontier.Seencheck.Seen(hash)
	}

	// Prepare GET request
	req, err := http.NewRequest("GET", item.URL.String(), nil)
	if err != nil {
		return outlinks, err
	}

	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept-Encoding", "*/*")
	if item.Hop > 0 && len(item.ParentItem.URL.String()) > 0 {
		req.Header.Set("Referer", item.ParentItem.URL.String())
	} else {
		req.Header.Set("Referer", item.URL.String())
	}

	// This retry loop is used for when the WARC writing fail,
	// it can happen when the response have an issue, such as
	// an unexpected EOF. That happens often when using proxies.
	for retryCount := 1; retryCount <= c.WARCRetry; retryCount++ {
		// Execute GET request
		c.URLsPerSecond.Incr(1)
		resp, err := c.Client.Do(req)
		if err != nil {
			return outlinks, err
		}
		defer resp.Body.Close()
		c.Crawled.Incr(1)

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

				// If a redirection is catched, then we execute the redirection
				// 1. Log the URL we just crawled
				// 2. Set the parent item to the item to the item we just crawled
				// 3. Parse the "location" header for the next URL to crawl
				// 4. Capture the URL
				// 5. Log the URL we just crawled
				if resp.StatusCode == 300 || resp.StatusCode == 301 ||
					resp.StatusCode == 302 || resp.StatusCode == 307 ||
					resp.StatusCode == 308 {

					c.logCrawlSuccess(executionStart, resp.StatusCode, item)

					if resp.Header.Get("location") == item.URL.String() || item.Redirect >= c.MaxRedirect {
						break
					}

					URL, err := url.Parse(resp.Header.Get("location"))
					if err != nil {
						return outlinks, err
					}

					newItem := frontier.NewItem(URL, item, item.Type, item.Hop)
					outlinks, err := c.captureWithGET(newItem)
					if err != nil {
						return outlinks, err
					}

					return outlinks, err
				}
			}
		}

		c.logCrawlSuccess(executionStart, resp.StatusCode, item)

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

			newAsset := frontier.NewItem(&asset, item, "asset", item.Hop)
			err = c.captureAsset(newAsset)
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
func (c *Crawl) Capture(item *frontier.Item) (outlinks []url.URL, err error) {
	// Check with HTTP HEAD request if the URL need a full headless browser or a simple GET request
	outlinks, err = c.captureWithGET(item)
	if err != nil {
		return nil, err
	}

	return outlinks, nil
}
