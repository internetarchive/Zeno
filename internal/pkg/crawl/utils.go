package crawl

import (
	"fmt"
	"hash/fnv"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/sirupsen/logrus"
)

var regexOutlinks *regexp.Regexp

func (c *Crawl) crawlSpeedLimiter() {
	maxConcurrentAssets := c.MaxConcurrentAssets

	for {
		if c.Client.WaitGroup.Size() > int(*c.ActiveWorkers)*8 {
			c.Paused.Set(true)
			c.Queue.Paused.Set(true)
		} else if c.Client.WaitGroup.Size() > int(*c.ActiveWorkers)*4 {
			c.MaxConcurrentAssets = 1
			c.Paused.Set(false)
			c.Queue.Paused.Set(false)
		} else {
			c.MaxConcurrentAssets = maxConcurrentAssets
			c.Paused.Set(false)
			c.Queue.Paused.Set(false)
		}

		time.Sleep(time.Second / 10)
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
			c.Queue.Paused.Set(true)
		} else {
			c.Paused.Set(false)
			c.Queue.Paused.Set(false)
		}

		time.Sleep(time.Second)
	}
}

func (c *Crawl) seencheckURL(URL string, URLType string) bool {
	h := fnv.New64a()
	h.Write([]byte(URL))
	hash := strconv.FormatUint(h.Sum64(), 10)

	found, _ := c.Seencheck.IsSeen(hash)
	if found {
		return true
	} else {
		c.Seencheck.Seen(hash, URLType)
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

// TODO: re-implement host limitation
// func (c *Crawl) shouldPause(host string) bool {
// 	return c.Frontier.GetActiveHostCount(host) >= c.MaxConcurrentRequestsPerDomain
// }

func isStatusCodeRedirect(statusCode int) bool {
	if statusCode == 300 || statusCode == 301 ||
		statusCode == 302 || statusCode == 307 ||
		statusCode == 308 {
		return true
	}
	return false
}
