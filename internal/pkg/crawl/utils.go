package crawl

import (
	"net/url"
	"regexp"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/sirupsen/logrus"
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
