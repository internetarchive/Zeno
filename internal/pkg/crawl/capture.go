package crawl

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/tidwall/gjson"
	"github.com/tomnomnom/linkheader"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
)

func (c *Crawl) executeGET(parentItem *frontier.Item, req *http.Request) (resp *http.Response, respPath string, err error) {
	var newItem *frontier.Item
	var newReq *http.Request
	var URL *url.URL

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
				return resp, respPath, err
			}
		} else {
			resp, err = c.ClientProxied.Do(req)
			if err != nil {
				if retry+2 > c.MaxRetry {
					return resp, respPath, err
				} else {
					logInfo.Println("Crucial error, retrying: " + err.Error())
					continue
				}
			}
		}

		if resp.StatusCode == 429 {
			sleepTime := time.Second * time.Duration(retry*2) // Retry after 0s, 2s, 4s, ... this could be tweaked in the future to be more customizable.
			logInfo.Println("429 error, retrying in " + sleepTime.String() + " second")
			time.Sleep(sleepTime)
			continue
		} else {
			break
		}
	}

	if c.Prometheus {
		c.PrometheusMetrics.DownloadedURI.Inc()
	}

	c.URIsPerSecond.Incr(1)
	c.Crawled.Incr(1)

	if c.UseHQ && parentItem.ID != "" {
		c.HQFinishedChannel <- parentItem
	}

	// If a redirection is catched, then we execute the redirection
	if isRedirection(resp.StatusCode) {
		if resp.Header.Get("location") == req.URL.String() || parentItem.Redirect >= c.MaxRedirect {
			return resp, respPath, nil
		}

		defer markTempFileDone(respPath)

		URL, err = url.Parse(resp.Header.Get("location"))
		if err != nil {
			return resp, respPath, err
		}

		newItem = frontier.NewItem(URL, parentItem, parentItem.Type, parentItem.Hop, "")
		newItem.Redirect = parentItem.Redirect + 1

		// Prepare GET request
		newReq, err = http.NewRequest("GET", URL.String(), nil)
		if err != nil {
			return resp, respPath, err
		}

		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Referer", newItem.ParentItem.URL.String())

		resp, respPath, err = c.executeGET(newItem, newReq)
		if err != nil {
			return resp, respPath, err
		}
	}
	return resp, respPath, nil
}

func (c *Crawl) captureAsset(item *frontier.Item, cookies []*http.Cookie) error {
	var executionStart = time.Now()
	var resp *http.Response

	// If --seencheck is enabled, then we check if the URI is in the
	// seencheck DB before doing anything. If it is in it, we skip the item
	if c.Seencheck {
		hash := strconv.FormatUint(item.Hash, 10)
		found, _ := c.Frontier.Seencheck.IsSeen(hash)
		if found {
			return nil
		}
		c.Frontier.Seencheck.Seen(hash, item.Type)
	}

	// Prepare GET request
	req, err := http.NewRequest("GET", item.URL.String(), nil)
	if err != nil {
		return err
	}

	req.Header.Set("Referer", item.ParentItem.URL.String())

	// Apply cookies obtained from the original URL captured
	for i := range cookies {
		req.AddCookie(cookies[i])
	}

	resp, respPath, err := c.executeGET(item, req)
	if err != nil {
		markTempFileDone(respPath)
		return err
	}
	defer resp.Body.Close()
	defer markTempFileDone(respPath)

	// needed for WARC writing
	io.Copy(io.Discard, resp.Body)

	c.logCrawlSuccess(executionStart, resp.StatusCode, item)

	return nil
}

// Capture capture the URL and return the outlinks
func (c *Crawl) Capture(item *frontier.Item) {
	var (
		executionStart = time.Now()
		resp           *http.Response
		waitGroup      sync.WaitGroup
	)

	defer waitGroup.Wait()

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

	// execute request
	resp, respPath, err := c.executeGET(item, req)
	if err != nil {
		logWarning.WithFields(logrus.Fields{
			"error": err,
		}).Warning(item.URL.String())
		markTempFileDone(respPath)
		return
	}
	defer resp.Body.Close()
	defer markTempFileDone(respPath)

	c.logCrawlSuccess(executionStart, resp.StatusCode, item)

	// Scrape potential URLs from Link HTTP header
	if item.Hop < c.MaxHops {
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
		go c.queueOutlinks(utils.StringSliceToURLSlice(discovered), item, &waitGroup)
	}

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

	// If the response isn't a text/*, we do not scrape it, and we delete the
	// temporary file if it exists
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/") {
		// enforce reading all data from the response
		io.Copy(io.Discard, resp.Body)
		return
	}

	// Turn the response into a doc that we will scrape
	var doc *goquery.Document
	if respPath != "" {
		file, err := os.Open(respPath)
		if err != nil {
			logWarning.WithFields(logrus.Fields{
				"error": err,
				"url":   item.URL.String(),
				"path":  respPath,
			}).Warning("Error opening temporary file for outlinks/assets extraction")
			return
		}

		doc, err = goquery.NewDocumentFromReader(file)
		if err != nil {
			logWarning.WithFields(logrus.Fields{
				"error": err,
				"url":   item.URL.String(),
				"path":  respPath,
			}).Warning("Error making goquery document from temporary file")
			return
		}
		_ = doc
		file.Close()
		markTempFileDone(respPath)
	} else {
		doc, err = goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			logWarning.WithFields(logrus.Fields{
				"error": err,
			}).Warning(item.URL.String())
			return
		}
		_ = doc
	}

	// Extract outlinks
	if item.Hop < c.MaxHops {
		outlinks, err := extractOutlinks(base, doc)
		if err != nil {
			logWarning.WithFields(logrus.Fields{
				"error": err,
			}).Warning(item.URL.String())
			return
		}

		waitGroup.Add(1)
		go c.queueOutlinks(outlinks, item, &waitGroup)
	}

	// Extract and capture assets
	assets, err := c.extractAssets(base, item, doc)
	if err != nil {
		logWarning.WithFields(logrus.Fields{
			"error": err,
		}).Warning(item.URL.String())
		return
	}

	c.Frontier.QueueCount.Incr(int64(len(assets)))
	var wg sync.WaitGroup
	for _, asset := range assets {
		c.Frontier.QueueCount.Incr(-1)

		// Just making sure we do not over archive
		if item.URL.String() == asset.String() {
			continue
		}

		// We ban googlevideo.com URLs because they are heavily rate limited by default, and
		// we don't want the crawler to spend an innapropriate amount of time archiving them
		// if strings.Contains(item.Host, "googlevideo.com") {
		// 	continue
		// }

		wg.Add(1)
		c.URIsPerSecond.Incr(1)
		go func(asset url.URL, wg *sync.WaitGroup) {
			defer wg.Done()

			newAsset := frontier.NewItem(&asset, item, "asset", item.Hop, "")
			err = c.captureAsset(newAsset, resp.Cookies())
			if err != nil {
				logWarning.WithFields(logrus.Fields{
					"error":          err,
					"queued":         c.Frontier.QueueCount.Value(),
					"crawled":        c.Crawled.Value(),
					"rate":           c.URIsPerSecond.Rate(),
					"active_workers": c.ActiveWorkers.Value(),
					"parent_hop":     item.Hop,
					"parent_url":     item.URL.String(),
					"type":           "asset",
				}).Warning(asset.String())
				return
			}
		}(asset, &wg)
	}

	wg.Wait()
}

func markTempFileDone(path string) {
	if path != "" {
		os.Rename(path, path+".done")
	}
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
