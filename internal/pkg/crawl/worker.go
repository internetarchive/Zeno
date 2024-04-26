package crawl

import (
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/utils"
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

// Worker is the key component of a crawl, it's a background processed dispatched
// when the crawl starts, it listens on a channel to get new URLs to archive,
// and eventually push newly discovered URLs back in the frontier.
func (c *Crawl) Worker() {
	defer c.WorkerPool.Done()

	// Start archiving the URLs!
	for item := range c.Frontier.PullChan {
		item := item

		// Check if the crawl is paused
		for c.Paused.Get() {
			time.Sleep(time.Second)
		}

		// If the host of the item is in the host exclusion list, we skip it
		if utils.StringInSlice(item.Host, c.ExcludedHosts) || !c.checkIncludedHosts(item.Host) {
			if c.UseHQ {
				// If we are using the HQ, we want to mark the item as done
				c.HQFinishedChannel <- item
			}

			continue
		}

		c.ActiveWorkers.Incr(1)
		c.Capture(item)
		c.ActiveWorkers.Incr(-1)
	}
}
