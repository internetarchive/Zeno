package crawl

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/remeh/sizedwaitgroup"
	"github.com/tidwall/gjson"
	"github.com/tomnomnom/linkheader"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
)

func (c *Crawl) executeGET(item *frontier.Item, req *http.Request) (resp *http.Response, err error) {
	var (
		executionStart = time.Now()
		newItem        *frontier.Item
		newReq         *http.Request
		URL            *url.URL
	)

	defer func() {
		if c.Prometheus {
			c.PrometheusMetrics.DownloadedURI.Inc()
		}

		c.URIsPerSecond.Incr(1)

		if item.Type == "seed" {
			c.CrawledSeeds.Incr(1)
		} else if item.Type == "asset" {
			c.CrawledAssets.Incr(1)
		}
	}()

	// Check if the crawl is paused
	for c.Paused.Get() {
		time.Sleep(time.Second)
	}

	// Retry on 429 error
	for retry := 0; retry < c.MaxRetry; retry++ {
		// Execute GET request
		if c.ClientProxied == nil || utils.StringContainsSliceElements(req.URL.Host, c.BypassProxy) {
			resp, err = c.Client.Do(req)
			if err != nil {
				if retry+1 >= c.MaxRetry {
					return resp, err
				}
			}
		} else {
			resp, err = c.ClientProxied.Do(req)
			if err != nil {
				if retry+1 >= c.MaxRetry {
					return resp, err
				}
			}
		}

		// This is unused unless there is an error or a 429.
		sleepTime := time.Second * time.Duration(retry*2) // Retry after 0s, 2s, 4s, ... this could be tweaked in the future to be more customizable.

		if err != nil {
			logInfo.WithFields(logrus.Fields{
				"url":         req.URL.String(),
				"retry_count": retry,
				"error":       err,
			}).Info("Crucial error, retrying...")

			time.Sleep(sleepTime)
			continue
		}

		if resp.StatusCode == 429 {
			logInfo.WithFields(logrus.Fields{
				"url":         req.URL.String(),
				"duration":    sleepTime.String(),
				"retry_count": retry,
				"status_code": resp.StatusCode,
			}).Info("We are being rate limited, sleeping then retrying..")

			// This ensures we aren't leaving the warc dialer hanging. Do note, 429s are filtered out by WARC writer regardless.
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			time.Sleep(sleepTime)
			continue
		} else {
			break
		}
	}

	c.logCrawlSuccess(executionStart, resp.StatusCode, item)

	// If a redirection is catched, then we execute the redirection
	if isRedirection(resp.StatusCode) {
		if resp.Header.Get("location") == req.URL.String() || item.Redirect >= c.MaxRedirect {
			return resp, nil
		}
		defer resp.Body.Close()

		// Needed for WARC writing
		// IMPORTANT! This will write redirects to WARC!
		io.Copy(io.Discard, resp.Body)

		URL, err = url.Parse(resp.Header.Get("location"))
		if err != nil {
			return resp, err
		}

		// Make URL absolute if they aren't.
		// Some redirects don't return full URLs, but rather, relative URLs. We would still like to follow these redirects.
		if !URL.IsAbs() {
			URL = req.URL.ResolveReference(URL)
		}

		// Seencheck the URL
		if c.Seencheck {
			found := c.seencheckURL(URL.String(), "seed")
			if found {
				return nil, errors.New("URL from redirection has already been seen")
			}
		} else if c.UseHQ {
			isNewURL, err := c.HQSeencheckURL(URL)
			if err != nil {
				return resp, err
			}

			if !isNewURL {
				return nil, errors.New("URL from redirection has already been seen")
			}
		}

		newItem = frontier.NewItem(URL, item, item.Type, item.Hop, item.ID)
		newItem.Redirect = item.Redirect + 1

		// Prepare GET request
		newReq, err = http.NewRequest("GET", URL.String(), nil)
		if err != nil {
			return resp, err
		}

		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Referer", newItem.ParentItem.URL.String())

		resp, err = c.executeGET(newItem, newReq)
		if err != nil {
			return resp, err
		}
	}

	return resp, nil
}

func (c *Crawl) captureAsset(item *frontier.Item, cookies []*http.Cookie) error {
	var resp *http.Response

	// Prepare GET request
	req, err := http.NewRequest("GET", item.URL.String(), nil)
	if err != nil {
		return err
	}

	req.Header.Set("Referer", item.ParentItem.URL.String())
	req.Header.Set("User-Agent", c.UserAgent)

	// Apply cookies obtained from the original URL captured
	for i := range cookies {
		req.AddCookie(cookies[i])
	}

	resp, err = c.executeGET(item, req)
	if err != nil && err.Error() == "URL from redirection has already been seen" {
		return nil
	} else if err != nil {
		logWarning.WithFields(logrus.Fields{
			"error": err,
		}).Warning(item.URL.String())
		return err
	}
	defer resp.Body.Close()

	// needed for WARC writing
	io.Copy(io.Discard, resp.Body)

	return nil
}

// Capture capture the URL and return the outlinks
func (c *Crawl) Capture(item *frontier.Item) {
	var (
		resp      *http.Response
		waitGroup sync.WaitGroup
	)

	defer func(i *frontier.Item) {
		waitGroup.Wait()

		if c.UseHQ && i.ID != "" {
			c.HQFinishedChannel <- i
		}
	}(item)

	// Prepare GET request
	req, err := http.NewRequest("GET", item.URL.String(), nil)
	if err != nil {
		logWarning.WithFields(logrus.Fields{
			"error": err,
		}).Warning(item.URL.String())
		return
	}

	if item.Hop > 0 && item.ParentItem != nil {
		req.Header.Set("Referer", item.ParentItem.URL.String())
	}

	req.Header.Set("User-Agent", c.UserAgent)

	if strings.Contains(item.URL.String(), "tiktok.com") {
		req.Header.Set("Authority", "www.tiktok.com")
		req.Header.Set("Sec-Ch-Ua", "\" Not A;Brand\";v=\"99\", \"Chromium\";v=\"99\", \"Microsoft Edge\";v=\"99\"")
		req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
		req.Header.Set("Sec-Ch-Ua-Platform", "\"Linux\"")
		req.Header.Set("Dnt", "1")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/99.0.4844.74 Safari/537.36 Edg/99.0.1150.52")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9")
		req.Header.Set("Sec-Fetch-Site", "none")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-User", "?1")
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9,fr;q=0.8")
	}

	// Execute request
	resp, err = c.executeGET(item, req)
	if err != nil && err.Error() == "URL from redirection has already been seen" {
		return
	} else if err != nil {
		logWarning.WithFields(logrus.Fields{
			"error": err,
		}).Warning(item.URL.String())
		return
	}
	defer resp.Body.Close()

	// Scrape potential URLs from Link HTTP header
	var (
		links      = linkheader.Parse(resp.Header.Get("link"))
		discovered []string
	)

	for _, link := range links {
		if link.Rel == "prev" || link.Rel == "next" {
			discovered = append(discovered, link.URL)
		}
	}

	waitGroup.Add(1)
	go c.queueOutlinks(utils.MakeAbsolute(item.URL, utils.StringSliceToURLSlice(discovered)), item, &waitGroup)

	// Store the base URL to turn relative links into absolute links later
	base, err := url.Parse(resp.Request.URL.String())
	if err != nil {
		logWarning.WithFields(logrus.Fields{
			"error": err,
		}).Warning(item.URL.String())
		return
	}

	// If the response is a JSON document, we would like to scrape it for links.
	if strings.Contains(resp.Header.Get("Content-Type"), "json") {
		jsonBody, err := io.ReadAll(resp.Body)
		if err != nil {
			logWarning.Warning(err)
			return
		}

		outlinks := getURLsFromJSON(gjson.ParseBytes(jsonBody))

		waitGroup.Add(1)
		go c.queueOutlinks(utils.MakeAbsolute(item.URL, utils.StringSliceToURLSlice(outlinks)), item, &waitGroup)

		return
	}

	// If the response isn't a text/*, we do not scrape it.
	// We also aren't going to scrape if assets and outlinks are turned off.
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/") || (c.DisableAssetsCapture && !c.DomainsCrawl && (c.MaxHops <= item.Hop)) {
		// Enforce reading all data from the response for WARC writing
		io.Copy(io.Discard, resp.Body)
		return
	}

	// Turn the response into a doc that we will scrape for outlinks and assets.
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		logWarning.WithFields(logrus.Fields{
			"error": err,
		}).Warning(item.URL.String())
		return
	}

	// Websites can use a <base> tag to specify a base for relative URLs in every other tags.
	// This checks for the "base" tag and resets the "base" URL variable with the new base URL specified
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/base
	if !utils.StringInSlice("base", c.DisabledHTMLTags) {
		oldBase := base

		doc.Find("base").Each(func(index int, goitem *goquery.Selection) {
			// If a new base got scraped, stop looking for one
			if oldBase != base {
				return
			}

			// Attempt to get a new base value from the base HTML tag
			link, exists := goitem.Attr("href")
			if exists {
				baseTagValue, err := url.Parse(link)
				if err != nil {
					logWarning.WithFields(logrus.Fields{
						"error": err,
					}).Warning(item.URL.String())
				} else {
					base = baseTagValue
				}
			}
		})
	}

	// Extract outlinks
	outlinks, err := extractOutlinks(base, doc)
	if err != nil {
		logWarning.WithFields(logrus.Fields{
			"error": err,
		}).Warning(item.URL.String())
		return
	}

	waitGroup.Add(1)
	go c.queueOutlinks(outlinks, item, &waitGroup)

	if c.DisableAssetsCapture {
		return
	}

	// Extract and capture assets
	assets, err := c.extractAssets(base, item, doc)
	if err != nil {
		logWarning.WithFields(logrus.Fields{
			"error": err,
		}).Warning(item.URL.String())
		return
	}

	// If we didn't find any assets, let's stop here
	if len(assets) == 0 {
		return
	}

	// If --local-seencheck is enabled, then we check if the assets are in the
	// seencheck DB. If they are, then they are skipped.
	// Else, if we use HQ, then we use HQ's seencheck.
	if c.Seencheck {
		seencheckedBatch := []url.URL{}
		for _, URL := range assets {
			found := c.seencheckURL(URL.String(), "asset")
			if found {
				continue
			} else {
				seencheckedBatch = append(seencheckedBatch, URL)
			}
		}

		if len(seencheckedBatch) == 0 {
			return
		}

		assets = seencheckedBatch
	} else if c.UseHQ {
		seencheckedURLs, err := c.HQSeencheckURLs(assets)
		// We ignore the error here because we don't want to slow down the crawl
		// if HQ is down or if the request failed. So if we get an error, we just
		// continue with the original list of assets.
		if err == nil {
			assets = seencheckedURLs
		}

		if len(assets) == 0 {
			return
		}
	}

	c.Frontier.QueueCount.Incr(int64(len(assets)))
	swg := sizedwaitgroup.New(c.MaxConcurrentAssets)
	for _, asset := range assets {
		c.Frontier.QueueCount.Incr(-1)

		// Just making sure we do not over archive by archiving the original URL
		if item.URL.String() == asset.String() {
			continue
		}

		// We ban googlevideo.com URLs because they are heavily rate limited by default, and
		// we don't want the crawler to spend an innapropriate amount of time archiving them
		if strings.Contains(item.Host, "googlevideo.com") {
			continue
		}

		swg.Add()
		c.URIsPerSecond.Incr(1)
		go func(asset url.URL, swg *sizedwaitgroup.SizedWaitGroup) {
			defer swg.Done()

			// Create the asset's item
			newAsset := frontier.NewItem(&asset, item, "asset", item.Hop, "")

			// Capture the asset
			err = c.captureAsset(newAsset, resp.Cookies())
			if err != nil {
				logWarning.WithFields(logrus.Fields{
					"error":          err,
					"queued":         c.Frontier.QueueCount.Value(),
					"crawled":        c.CrawledSeeds.Value() + c.CrawledAssets.Value(),
					"rate":           c.URIsPerSecond.Rate(),
					"active_workers": c.ActiveWorkers.Value(),
					"parent_hop":     item.Hop,
					"parent_url":     item.URL.String(),
					"type":           "asset",
				}).Warning(asset.String())
				return
			}

			// If we made it to this point, it means that the asset have been crawled successfully,
			// then we can increment the locallyCrawled variable
			atomic.AddUint64(&item.LocallyCrawled, 1)
		}(asset, &swg)
	}

	swg.Wait()
}

func getURLsFromJSON(payload gjson.Result) (links []string) {
	if payload.IsArray() {
		for _, arrayElement := range payload.Array() {
			links = append(links, getURLsFromJSON(arrayElement)...)
		}
	} else {
		for _, element := range payload.Map() {
			if element.IsObject() {
				links = append(links, getURLsFromJSON(element)...)
			} else if element.IsArray() {
				links = append(links, getURLsFromJSON(element)...)
			} else {
				if strings.HasPrefix(element.Str, "http") || strings.HasPrefix(element.Str, "/") {
					links = append(links, element.Str)
				}
			}
		}
	}

	return links
}
