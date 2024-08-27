package crawl

import (
	"net/url"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/internal/pkg/queue"
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

func (c *Crawl) queueOutlinks(outlinks []*url.URL, item *queue.Item, wg *sync.WaitGroup) {
	defer wg.Done()

	var excluded bool

	// Send the outlinks to the pool of workers
	var items = make([]*queue.Item, 0, len(outlinks))
	for _, outlink := range outlinks {
		outlink := outlink

		// If the host of the outlink is in the host exclusion list, or the host is not in the host inclusion list
		// if one is specified, we ignore the outlink
		if utils.StringInSlice(outlink.Host, c.ExcludedHosts) || !c.checkIncludedHosts(outlink.Host) {
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

		// Seencheck the outlink
		if c.UseSeencheck {
			if c.Seencheck.SeencheckURL(utils.URLToString(outlink), "seed") {
				continue
			}
		}

		if c.DomainsCrawl && strings.Contains(item.URL.Host, outlink.Host) && item.Hop == 0 {
			newItem, err := queue.NewItem(outlink, item.URL, "seed", 0, "", false)
			if err != nil {
				c.Log.WithFields(c.genLogFields(err, outlink, nil)).Error("unable to create new item from outlink, discarding")
				continue
			}

			if c.UseHQ {
				c.HQProducerChannel <- newItem
			} else {
				items = append(items, newItem)
			}
		} else if uint64(c.MaxHops) >= item.Hop+1 {
			newItem, err := queue.NewItem(outlink, item.URL, "seed", item.Hop+1, "", false)
			if err != nil {
				c.Log.WithFields(c.genLogFields(err, outlink, nil)).Error("unable to create new item from outlink, discarding")
				continue
			}

			if c.UseHQ {
				c.HQProducerChannel <- newItem
			} else {
				items = append(items, newItem)
			}
		}
	}

	if !c.UseHQ && len(items) > 0 {
		err := c.Queue.BatchEnqueue(items...)
		if err != nil {
			c.Log.Error("unable to enqueue outlinks, discarding", "error", err)
		}
	}
}
