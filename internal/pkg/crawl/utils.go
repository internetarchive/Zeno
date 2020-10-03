package crawl

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
)

var regexOutlinks *regexp.Regexp

func (crawl *Crawl) setupCloseHandler() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		logrus.Warning("CTRL+C catched.. cleaning up and exiting.")
		crawl.Finish()
		os.Exit(0)
	}()
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

func extractOutlinksGoquery(resp *http.Response) (outlinks []url.URL, err error) {
	var rawOutlinks []string

	// Store the base URL to turn relative links into absolute links later
	base, err := url.Parse(resp.Request.URL.String())
	if err != nil {
		log.Fatal(err)
	}

	// Turn the response into a doc that we will scrape
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return outlinks, err
	}

	// Extract assets and outlinks from elements
	doc.Find("a").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("href")
		if exists {
			rawOutlinks = append(rawOutlinks, link)
		}
	})
	doc.Find("img").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("src")
		if exists {
			rawOutlinks = append(rawOutlinks, link)
		}
	})
	doc.Find("video").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("src")
		if exists {
			rawOutlinks = append(rawOutlinks, link)
		}
	})
	doc.Find("script").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("src")
		if exists {
			rawOutlinks = append(rawOutlinks, link)
		}
	})
	doc.Find("link").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("href")
		if exists {
			rawOutlinks = append(rawOutlinks, link)
		}
	})

	// Dedupe outlinks discovered
	rawOutlinks = utils.DedupeStringSlice(rawOutlinks)

	// Turn all outlinks into url.URL
	for _, outlink := range rawOutlinks {
		decodedOutlink, err := url.QueryUnescape(outlink)
		if err != nil {
			logrus.Warning("Unable to parse outlink: " + decodedOutlink)
			continue
		}

		URL, err := url.Parse(decodedOutlink)
		if err != nil {
			continue
		}

		outlinks = append(outlinks, *URL)
	}

	// Extract all text on the page and extract the outlinks from it
	//textOutlinks := extractOutlinksRegex(doc.Find("body").Text())
	//for _, link := range textOutlinks {
	//	outlinks = append(outlinks, link)
	//}

	// Go over all outlinks and make sure they are absolute links
	for i, outlink := range outlinks {
		outlinks[i] = *base.ResolveReference(&outlink)
	}

	return outlinks, nil
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
