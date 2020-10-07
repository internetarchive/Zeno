package crawl

import (
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
)

var regexOutlinks *regexp.Regexp

func (crawl *Crawl) writeFrontierToDisk() {
	for !crawl.Finished.Get() {
		crawl.Frontier.Save()
		time.Sleep(time.Minute * 1)
	}
}

func (crawl *Crawl) setupCloseHandler() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	logrus.Warning("CTRL+C catched.. cleaning up and exiting.")
	close(c)
	crawl.Finish()
	os.Exit(0)
}

func needBrowser(item *frontier.Item) bool {
	res, err := http.Head(item.URL.String())
	if err != nil {
		return true
	}

	// If the Content-Type is text/html, we use a headless browser
	contentType := res.Header.Get("content-type")
	if strings.Contains(contentType, "text/html") {
		return true
	}

	return false
}

func extractAssets(resp *http.Response) (assets []url.URL, err error) {
	var rawAssets []string

	// Store the base URL to turn relative URLs into absolute URLs
	base, err := url.Parse(resp.Request.URL.String())
	if err != nil {
		return assets, err
	}

	// Turn the response into a doc that we will scrape
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return assets, err
	}

	// Extract assets on the page (images, scripts, videos..)
	doc.Find("img").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("src")
		if exists {
			rawAssets = append(rawAssets, link)
		}
	})
	doc.Find("video").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("src")
		if exists {
			rawAssets = append(rawAssets, link)
		}
	})
	doc.Find("script").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("src")
		if exists {
			rawAssets = append(rawAssets, link)
		}
	})
	doc.Find("link").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("href")
		if exists {
			rawAssets = append(rawAssets, link)
		}
	})
	doc.Find("audio").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("src")
		if exists {
			rawAssets = append(rawAssets, link)
		}
	})
	doc.Find("iframe").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("src")
		if exists {
			rawAssets = append(rawAssets, link)
		}
	})

	// Dedupe assets discovered and turn them into url.URL
	assets = stringSliceToURLSlice(rawAssets)

	// Go over all assets and outlinks and make sure they are absolute links
	assets = makeAbsolute(base, assets)

	return assets, nil
}

func extractOutlinks(resp *http.Response) (outlinks []url.URL, err error) {
	var rawOutlinks []string

	// Store the base URL to turn relative links into absolute links later
	base, err := url.Parse(resp.Request.URL.String())
	if err != nil {
		return outlinks, err
	}

	// Turn the response into a doc that we will scrape
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return outlinks, err
	}

	// Extract outlinks
	doc.Find("a").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("href")
		if exists {
			rawOutlinks = append(rawOutlinks, link)
		}
	})

	// Dedupe outlinks discovered and turn them into url.URL
	outlinks = stringSliceToURLSlice(rawOutlinks)

	// Extract all text on the page and extract the outlinks from it
	textOutlinks := extractOutlinksRegex(doc.Find("body").Text())
	for _, link := range textOutlinks {
		outlinks = append(outlinks, link)
	}

	// Go over all outlinks and make sure they are absolute links
	outlinks = makeAbsolute(base, outlinks)

	return outlinks, nil
}

func stringSliceToURLSlice(rawURLs []string) (URLs []url.URL) {
	rawURLs = utils.DedupeStringSlice(rawURLs)

	for _, URL := range rawURLs {
		decodedURL, err := url.QueryUnescape(URL)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error":    err,
				"outlinks": URL,
			}).Debug("Unable to parse outlink")
			continue
		}

		URL, err := url.Parse(decodedURL)
		if err != nil {
			continue
		}

		URLs = append(URLs, *URL)
	}

	return URLs
}

func makeAbsolute(base *url.URL, URLs []url.URL) []url.URL {
	for i, URL := range URLs {
		if URL.IsAbs() == false {
			URLs[i] = *base.ResolveReference(&URL)
		}
	}

	return URLs
}

func extractOutlinksRegex(source string) (outlinks []url.URL) {
	// Extract outlinks and dedupe them
	rawOutlinks := utils.DedupeStringSlice(regexOutlinks.FindAllString(source, -1))

	// Validate outlinks
	for _, outlink := range rawOutlinks {
		URL, err := url.Parse(outlink)
		if err != nil {
			continue
		}
		err = utils.ValidateURL(URL)
		if err != nil {
			continue
		}
		outlinks = append(outlinks, *URL)
	}

	return outlinks
}
