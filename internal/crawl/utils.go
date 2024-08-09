package crawl

import (
	"fmt"
	"net/url"
	"time"

	"github.com/internetarchive/Zeno/internal/hq"
	"github.com/internetarchive/Zeno/internal/stats"
	"github.com/internetarchive/Zeno/internal/utils"
	"github.com/sirupsen/logrus"
)

const (
	// B represent a Byte
	B = 1
	// KB represent a Kilobyte
	KB = 1024 * B
	// MB represent a MegaByte
	MB = 1024 * KB
	// GB represent a GigaByte
	GB = 1024 * MB
)

// TODO: re-implement host limitation
// func (c *Crawl) crawlSpeedLimiter() {
// 	maxConcurrentAssets := c.MaxConcurrentAssets

// 	for {
// 		if c.Client.WaitGroup.Size() > c.Workers*8 {
// 			c.Paused.Set(true)
// 			c.Frontier.Paused.Set(true)
// 		} else if c.Client.WaitGroup.Size() > c.Workers*4 {
// 			c.MaxConcurrentAssets = 1
// 			c.Paused.Set(false)
// 			c.Frontier.Paused.Set(false)
// 		} else {
// 			c.MaxConcurrentAssets = maxConcurrentAssets
// 			c.Paused.Set(false)
// 			c.Frontier.Paused.Set(false)
// 		}

// 		time.Sleep(time.Second / 4)
// 	}
// }

func (c *Crawl) CheckIncludedHosts(host string) bool {
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
			hq.Paused.CompareAndSwap(false, true)
			c.Queue.Paused.Set(true)
			c.Workers.Pause <- struct{}{}
			stats.SetCrawlState("paused")
		} else {
			c.Paused.Set(false)
			hq.Paused.CompareAndSwap(true, false)
			c.Queue.Paused.Set(false)
			c.Workers.Unpause <- struct{}{}
			if stats.GetCrawlState() == "paused" {
				stats.SetCrawlState("running")
			}
		}

		time.Sleep(time.Second)
	}
}

func (c *Crawl) excludeHosts(URLs []*url.URL) (output []*url.URL) {
	for _, URL := range URLs {
		if utils.StringInSlice(URL.Host, c.ExcludedHosts) || !c.CheckIncludedHosts(URL.Host) {
			continue
		} else {
			output = append(output, URL)
		}
	}

	return output
}

// TODO: re-implement host limitation
// func (c *Crawl) shouldPause(host string) bool {
// 	return c.Frontier.GetActiveHostCount(host) >= c.MaxConcurrentRequestsPerDomain
// }