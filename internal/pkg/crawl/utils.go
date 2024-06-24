package crawl

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/utils"
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

func (c *Crawl) checkIncludedHosts(host string) bool {
	// If no hosts are included, all hosts are included
	if len(c.IncludedHosts) == 0 {
		return true
	}

	return utils.StringInSlice(host, c.IncludedHosts)
}

func (c *Crawl) handleCrawlPause() {
	for {
		spaceLeft := float64(utils.GetFreeDiskSpace(c.JobPath).Avail) / float64(GB)
		if spaceLeft <= float64(c.MinSpaceRequired) {
			logrus.Errorln(fmt.Sprintf("Not enough disk space: %d GB required, %f GB available. "+
				"Please free some space for the crawler to resume.", c.MinSpaceRequired, spaceLeft))
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
		if utils.StringInSlice(URL.Host, c.ExcludedHosts) || !c.checkIncludedHosts(URL.Host) {
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
	return c.Frontier.GetActiveHostCount(host) >= c.MaxConcurrentRequestsPerDomain
}

func isStatusCodeRedirect(statusCode int) bool {
	if statusCode == 300 || statusCode == 301 ||
		statusCode == 302 || statusCode == 307 ||
		statusCode == 308 {
		return true
	}
	return false
}
