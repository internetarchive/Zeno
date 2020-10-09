package crawl

import (
	"net/url"
	"strings"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
)

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
