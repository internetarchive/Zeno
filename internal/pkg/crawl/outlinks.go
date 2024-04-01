package crawl

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/internal/pkg/frontier"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
)

func extractOutlinks(base *url.URL, doc *goquery.Document) (outlinks []*url.URL, err error) {
	var rawOutlinks []string

	// Extract outlinks
	doc.Find("a").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("href")
		if exists {
			rawOutlinks = append(rawOutlinks, link)
		}
	})

	// Extract iframes as 'outlinks' as they usually can be treated as entirely seperate pages with entirely seperate assets.
	doc.Find("iframe").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("src")
		if exists {
			rawOutlinks = append(rawOutlinks, link)
		}
	})

	doc.Find("ref").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("target")
		if exists {
			rawOutlinks = append(rawOutlinks, link)
		}

		fmt.Println(item.Text())
	})

	// Turn strings into url.URL
	outlinks = utils.StringSliceToURLSlice(rawOutlinks)

	// Extract all text on the page and extract the outlinks from it
	textOutlinks := extractLinksFromText(doc.Find("body").RemoveFiltered("script").Text())
	outlinks = append(outlinks, textOutlinks...)

	// Go over all outlinks and make sure they are absolute links
	outlinks = utils.MakeAbsolute(base, outlinks)

	// Hash (or fragment) URLs are navigational links pointing to the exact same page as such, they should not be treated as new outlinks.
	outlinks = utils.RemoveFragments(outlinks)

	return utils.DedupeURLs(outlinks), nil
}

func (c *Crawl) queueOutlinks(outlinks []*url.URL, item *frontier.Item, wg *sync.WaitGroup) {
	defer wg.Done()

	var excluded bool

	// Send the outlinks to the pool of workers
	for _, outlink := range outlinks {
		outlink := outlink

		// If the host of the outlink is in the host exclusion list, we ignore it
		if utils.StringInSlice(outlink.Host, c.ExcludedHosts) {
			continue
		}

		// If the outlink match any excluded string, we ignore it
		for _, excludedString := range c.ExcludedStrings {
			if strings.Contains(utils.URLToString(outlink), excludedString) {
				excluded = true
				break
			}
		}

		if excluded {
			excluded = false
			continue
		}

		if c.DomainsCrawl && strings.Contains(item.Host, outlink.Host) && item.Hop == 0 {
			newItem := frontier.NewItem(outlink, item, "seed", 0, "", utils.Pointer(false))
			if c.UseHQ {
				c.HQProducerChannel <- newItem
			} else {
				c.Frontier.PushChan <- newItem
			}
		} else if c.MaxHops >= item.Hop+1 {
			newItem := frontier.NewItem(outlink, item, "seed", item.Hop+1, "", utils.Pointer(false))
			if c.UseHQ {
				c.HQProducerChannel <- newItem
			} else {
				c.Frontier.PushChan <- newItem
			}
		}
	}
}
