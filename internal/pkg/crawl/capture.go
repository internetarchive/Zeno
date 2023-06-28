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

	"github.com/CorentinB/Zeno/internal/pkg/crawl/sitespecific/cloudflarestream"
	"github.com/CorentinB/Zeno/internal/pkg/crawl/sitespecific/tiktok"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/PuerkitoBio/goquery"
	"github.com/remeh/sizedwaitgroup"
	"github.com/tidwall/gjson"
	"github.com/tomnomnom/linkheader"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
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

	// Temporarily pause crawls for individual hosts if they are over our configured maximum concurrent requests per domain.
	for c.shouldPause(item.Host) {
		time.Sleep(time.Millisecond * time.Duration(c.ConcurrentSleepTime))
	}

	c.Frontier.CrawlPool.Incr(item.Host)

	defer c.Frontier.CrawlPool.Decr(item.Host)
	// todo: validate that this is decremented even on 429.

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
			logError.WithFields(c.genLogFields(err, req.URL, nil)).Error("error while executing GET request, retrying")

			time.Sleep(sleepTime)

			continue
		}

		if resp.StatusCode == 429 {
			logWarning.WithFields(c.genLogFields(err, req.URL, map[string]interface{}{
				"sleepTime":  sleepTime.String(),
				"retryCount": retry,
				"statusCode": resp.StatusCode,
			})).Warn("we are being rate limited, sleeping then retrying..")

			// This ensures we aren't leaving the warc dialer hanging.
			// Do note, 429s are filtered out by WARC writer regardless.
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
		if resp.Header.Get("location") == utils.URLToString(req.URL) || item.Redirect >= c.MaxRedirect {
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
			found := c.seencheckURL(utils.URLToString(URL), "seed")
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
		newReq, err = http.NewRequest("GET", utils.URLToString(URL), nil)
		if err != nil {
			return resp, err
		}

		// Set new request headers on the new request :(
		newReq.Header.Set("User-Agent", c.UserAgent)
		newReq.Header.Set("Referer", utils.URLToString(newItem.ParentItem.URL))

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
	req, err := http.NewRequest("GET", utils.URLToString(item.URL), nil)
	if err != nil {
		return err
	}

	req.Header.Set("Referer", utils.URLToString(item.ParentItem.URL))
	req.Header.Set("User-Agent", c.UserAgent)

	// Apply cookies obtained from the original URL captured
	for i := range cookies {
		req.AddCookie(cookies[i])
	}

	resp, err = c.executeGET(item, req)
	if err != nil && err.Error() == "URL from redirection has already been seen" {
		return nil
	} else if err != nil {
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
	req, err := http.NewRequest("GET", utils.URLToString(item.URL), nil)
	if err != nil {
		logError.WithFields(c.genLogFields(err, item.URL, nil)).Error("error while preparing GET request")
		return
	}

	if item.Hop > 0 && item.ParentItem != nil {
		req.Header.Set("Referer", utils.URLToString(item.ParentItem.URL))
	}

	req.Header.Set("User-Agent", c.UserAgent)

	// Execute site-specific code on the request, before sending it
	if strings.Contains(item.URL.Host, "tiktok.com") {
		req = tiktok.AddHeaders(req)
	}

	// Execute request
	resp, err = c.executeGET(item, req)
	if err != nil && err.Error() == "URL from redirection has already been seen" {
		return
	} else if err != nil {
		logError.WithFields(c.genLogFields(err, item.URL, nil)).Error("error while executing GET request")
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
	base, err := url.Parse(utils.URLToString(resp.Request.URL))
	if err != nil {
		logError.WithFields(c.genLogFields(err, item.URL, nil)).Error("error while parsing base URL")
		return
	}

	// If the response is a JSON document, we would like to scrape it for links.
	if strings.Contains(resp.Header.Get("Content-Type"), "json") {
		jsonBody, err := io.ReadAll(resp.Body)
		if err != nil {
			logError.WithFields(c.genLogFields(err, item.URL, nil)).Error("error while reading JSON body")
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
		_, err := io.Copy(io.Discard, resp.Body)
		if err != nil {
			logError.WithFields(c.genLogFields(err, item.URL, nil)).Error("error while reading response body")
		}

		return
	}

	// Turn the response into a doc that we will scrape for outlinks and assets.
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		logError.WithFields(c.genLogFields(err, item.URL, nil)).Error("error while creating goquery document")
		return
	}

	// Execute site-specific code on the document
	if strings.Contains(base.Host, "cloudflarestream.com") {
		// Look for JS files necessary for the playback of the video
		cfstreamURLs, err := cloudflarestream.GetJSFiles(doc, base, *c.Client)
		if err != nil {
			logError.WithFields(c.genLogFields(err, item.URL, nil)).Error("error while getting JS files from cloudflarestream")
			return
		}

		// Seencheck the URLs we captured, we ignore the returned value here
		// because we already archived the URLs, we just want them to be added
		// to the seencheck table.
		if c.Seencheck {
			for _, cfstreamURL := range cfstreamURLs {
				c.seencheckURL(cfstreamURL, "asset")
			}
		} else if c.UseHQ {
			_, err := c.HQSeencheckURLs(utils.StringSliceToURLSlice(cfstreamURLs))
			if err != nil {
				logError.WithFields(c.genLogFields(err, item.URL, map[string]interface{}{
					"urls": cfstreamURLs,
				})).Error("error while seenchecking assets via HQ")
			}
		}

		// Log the archived URLs
		for _, cfstreamURL := range cfstreamURLs {
			logInfo.WithFields(c.genLogFields(err, cfstreamURL, map[string]interface{}{
				"parentHop": item.Hop,
				"parentUrl": utils.URLToString(item.URL),
				"type":      "asset",
			})).Info("URL archived")
		}
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
					logError.WithFields(c.genLogFields(err, item.URL, nil)).Error("error while parsing base tag value")
				} else {
					base = baseTagValue
				}
			}
		})
	}

	// Extract outlinks
	outlinks, err := extractOutlinks(base, doc)
	if err != nil {
		logError.WithFields(c.genLogFields(err, item.URL, nil)).Error("error while extracting outlinks")
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
		logError.WithFields(c.genLogFields(err, item.URL, nil)).Error("error while extracting assets")
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
		seencheckedBatch := []*url.URL{}

		for _, URL := range assets {
			found := c.seencheckURL(utils.URLToString(URL), "asset")
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
		if err != nil {
			logError.WithFields(c.genLogFields(err, nil, map[string]interface{}{
				"urls":      assets,
				"parentHop": item.Hop,
				"parentUrl": utils.URLToString(item.URL),
			})).Error("error while seenchecking assets via HQ")
		} else {
			assets = seencheckedURLs
		}

		if len(assets) == 0 {
			return
		}
	}

	c.Frontier.QueueCount.Incr(int64(len(assets)))
	swg := sizedwaitgroup.New(c.MaxConcurrentAssets)
	excluded := true

	for _, asset := range assets {
		c.Frontier.QueueCount.Incr(-1)

		// Just making sure we do not over archive by archiving the original URL
		if utils.URLToString(item.URL) == utils.URLToString(asset) {
			continue
		}

		// We ban googlevideo.com URLs because they are heavily rate limited by default, and
		// we don't want the crawler to spend an innapropriate amount of time archiving them
		if strings.Contains(item.Host, "googlevideo.com") {
			continue
		}

		// If the URL match any excluded string, we ignore it
		for _, excludedString := range c.ExcludedStrings {
			if strings.Contains(utils.URLToString(asset), excludedString) {
				excluded = true
				break
			}
		}

		if excluded {
			excluded = false
			continue
		}

		swg.Add()
		c.URIsPerSecond.Incr(1)
		go func(asset *url.URL, swg *sizedwaitgroup.SizedWaitGroup) {
			defer swg.Done()

			// Create the asset's item
			newAsset := frontier.NewItem(asset, item, "asset", item.Hop, "")

			// Capture the asset
			err = c.captureAsset(newAsset, resp.Cookies())
			if err != nil {
				logError.WithFields(c.genLogFields(err, &asset, map[string]interface{}{
					"parentHop": item.Hop,
					"parentUrl": utils.URLToString(item.URL),
					"type":      "asset",
				})).Error("error while capturing asset")
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
