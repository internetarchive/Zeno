package crawl

import "github.com/remeh/sizedwaitgroup"

// Worker is the key component of a crawl, it's a background processed dispatched
// when the crawl starts, it listens on a channel to get new URLs to archive,
// and eventually push newly discovered URLs back in the frontier.
func (c *Crawl) Worker(wg *sizedwaitgroup.SizedWaitGroup) {
	// Start archiving the URLs!
	for item := range c.Frontier.PullChan {
		item := item

		c.ActiveWorkers.Incr(1)
		c.Capture(item)
		c.ActiveWorkers.Incr(-1)
	}

	wg.Done()
}
