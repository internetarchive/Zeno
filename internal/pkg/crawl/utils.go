package crawl

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/zeebo/xxh3"
)

var regexOutlinks *regexp.Regexp

func (c *Crawl) writeFrontierToDisk() {
	for !c.Finished.Get() {
		c.Frontier.Save()
		time.Sleep(time.Minute * 1)
	}
}

func (c *Crawl) crawlSpeedLimiter() {
	maxConcurrentAssets := c.MaxConcurrentAssets

	for {
		if c.Client.WaitGroup.Size() > c.Workers*8 {
			c.Paused.Set(true)
			c.Frontier.Paused.Set(true)
		} else if c.Client.WaitGroup.Size() > c.Workers*4 {
			c.MaxConcurrentAssets = 1
			c.Paused.Set(false)
			c.Frontier.Paused.Set(false)
		} else {
			c.MaxConcurrentAssets = maxConcurrentAssets
			c.Paused.Set(false)
			c.Frontier.Paused.Set(false)
		}

		time.Sleep(time.Second / 4)
	}
}

func (c *Crawl) handleCrawlPause() {
	for {
		if float64(utils.GetFreeDiskSpace(c.JobPath).Avail)/float64(GB) <= 20 {
			logrus.Errorln("Not enough disk space. Please free some space and restart the crawler.")
			c.Paused.Set(true)
			c.Frontier.Paused.Set(true)
		} else {
			c.Paused.Set(false)
			c.Frontier.Paused.Set(false)
		}

		time.Sleep(time.Second)
	}
}

func (c *Crawl) seencheckURL(url string, urlType string) bool {
	hash := strconv.FormatUint(xxh3.HashString(url), 10)
	found, _ := c.Frontier.Seencheck.IsSeen(hash)
	if found {
		return true
	} else {
		c.Frontier.Seencheck.Seen(hash, urlType)
		return false
	}
}

func (c *Crawl) excludeHosts(URLs []url.URL) (output []url.URL) {
	for _, url := range URLs {
		if utils.StringInSlice(url.Host, c.ExcludedHosts) {
			continue
		} else {
			output = append(output, url)
		}
	}

	return output
}

func extractLinksFromText(source string) (links []url.URL) {
	// Extract links and dedupe them
	rawLinks := utils.DedupeStrings(regexOutlinks.FindAllString(source, -1))

	// Validate links
	for _, link := range rawLinks {
		URL, err := url.Parse(link)
		if err != nil {
			continue
		}
		err = utils.ValidateURL(URL)
		if err != nil {
			continue
		}
		links = append(links, *URL)
	}

	return links
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
