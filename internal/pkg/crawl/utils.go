package crawl

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
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

func extractAssets(base *url.URL, doc *goquery.Document) (assets []url.URL, err error) {
	var rawAssets []string

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

	// Turn strings into url.URL
	assets = stringSliceToURLSlice(rawAssets)

	// Go over all assets and outlinks and make sure they are absolute links
	assets = makeAbsolute(base, assets)

	return utils.DedupeURLs(assets), nil
}

func extractOutlinks(base *url.URL, doc *goquery.Document) (outlinks []url.URL, err error) {
	var rawOutlinks []string

	// Extract outlinks
	doc.Find("a").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("href")
		if exists {
			rawOutlinks = append(rawOutlinks, link)
		}
	})

	// Turn strings into url.URL
	outlinks = stringSliceToURLSlice(rawOutlinks)

	// Extract all text on the page and extract the outlinks from it
	textOutlinks := extractOutlinksRegex(doc.Find("body").Text())
	for _, link := range textOutlinks {
		outlinks = append(outlinks, link)
	}

	// Go over all outlinks and make sure they are absolute links
	outlinks = makeAbsolute(base, outlinks)

	return utils.DedupeURLs(outlinks), nil
}

func stringSliceToURLSlice(rawURLs []string) (URLs []url.URL) {
	for _, URL := range rawURLs {
		decodedURL, err := url.QueryUnescape(URL)
		if err != nil {
			logWarning.WithFields(logrus.Fields{
				"error":    err,
				"outlinks": URL,
			}).Warning("Unable to parse outlink")
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
	rawOutlinks := utils.DedupeStrings(regexOutlinks.FindAllString(source, -1))

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
