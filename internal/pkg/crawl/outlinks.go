package crawl

import (
	"net/url"
	"strings"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/PuerkitoBio/goquery"
)

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
	outlinks = utils.StringSliceToURLSlice(rawOutlinks)

	// Extract all text on the page and extract the outlinks from it
	textOutlinks := extractOutlinksRegex(doc.Find("body").Text())
	for _, link := range textOutlinks {
		outlinks = append(outlinks, link)
	}

	// Go over all outlinks and make sure they are absolute links
	outlinks = utils.MakeAbsolute(base, outlinks)

	return utils.DedupeURLs(outlinks), nil
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

func (c *Crawl) queueOutlinks(outlinks []url.URL, item *frontier.Item) {
	// Send the outlinks to the pool of workers
	for _, outlink := range outlinks {
		outlink := outlink
		if c.DomainsCrawl && strings.Contains(item.Host, outlink.Host) && item.Hop == 0 {
			newItem := frontier.NewItem(&outlink, item, "seed", 0)
			if c.UseKafka && len(c.KafkaOutlinksTopic) > 0 {
				c.KafkaProducerChannel <- newItem
			} else {
				c.Frontier.PushChan <- newItem
			}
		} else {
			newItem := frontier.NewItem(&outlink, item, "seed", item.Hop+1)
			if c.UseKafka && len(c.KafkaOutlinksTopic) > 0 {
				c.KafkaProducerChannel <- newItem
			} else {
				c.Frontier.PushChan <- newItem
			}
		}
	}
}
