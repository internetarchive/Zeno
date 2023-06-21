package crawl

import (
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/sirupsen/logrus"
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

func (c *Crawl) seencheckURL(URL string, URLType string) bool {
	hash := strconv.FormatUint(xxh3.HashString(URL), 10)
	found, _ := c.Frontier.Seencheck.IsSeen(hash)
	if found {
		return true
	} else {
		c.Frontier.Seencheck.Seen(hash, URLType)
		return false
	}
}

func (c *Crawl) excludeHosts(URLs []*url.URL) (output []*url.URL) {
	for _, URL := range URLs {
		if utils.StringInSlice(URL.Host, c.ExcludedHosts) {
			continue
		} else {
			output = append(output, URL)
		}
	}

	return output
}

func extractLinksFromText(source string) (links []*url.URL) {
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
		links = append(links, URL)
	}

	return links
}

func (c *Crawl) shouldPause(host string) bool {
	if int(c.Frontier.CrawlPool.GetCount(host)) >= c.MaxConcurrentRequestsPerDomain {
		return true
	} else {
		return false
	}
}
