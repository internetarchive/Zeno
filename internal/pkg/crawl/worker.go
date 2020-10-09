package crawl

import (
	"strconv"
	"strings"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/sirupsen/logrus"
)

// Worker archive the items!
func (c *Crawl) Worker(item *frontier.Item) {
	// Capture the page
	outlinks, err := c.Capture(item)
	if err != nil {
		log.WithFields(logrus.Fields{
			"error": err,
		}).Warning(item.URL.String())
		return
	}

	// Send the outlinks to the pool of workers
	if item.Hop < c.MaxHops {
		for _, outlink := range outlinks {
			// If --seencheck is enabled, then we check if the URI is in the
			// seencheck DB before doing anything. If it is in it, we skip the item
			if c.Frontier.UseSeencheck {
				hash := strconv.FormatUint(item.Hash, 10)
				if c.Frontier.Seencheck.IsSeen(hash) {
					continue
				}
				c.Frontier.Seencheck.Seen(hash)
			}

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
}
