package crawl

import (
	"net/url"
	"time"

	"git.archive.org/wb/gocrawlhq"
	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/sirupsen/logrus"
)

type hqMessage struct {
	URL       string `json:"u"`
	HopsCount uint8  `json:"hop"`
	ParentURL string `json:"parent_url"`
}

func (c *Crawl) hqProducer() {
	crawlHQClient, err := gocrawlhq.Init(c.HQKey, c.HQSecret, c.HQProject, c.HQAddress)
	if err != nil {
		logrus.Panic(err)
	}

	for item := range c.HQProducerChannel {
	send:
		if c.Finished.Get() {
			break
		}

		// var newHQMsg = new(gocrawlhq.DiscoveredPayload)

		// newHQMsg.URLs = []gocrawlhq.URL{
		// 	gocrawlhq.URL{
		// 		ID:    item.ID,
		// 		Value: item.URL.String(),
		// 		Path:  "",
		// 		Via:   item.ParentItem.URL.String(),
		// 	},
		// }

		// newHQMsg.Type = item.Type

		_, err := crawlHQClient.Discovered([]string{item.URL.String()}, item.Type, false)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"project": c.HQProject,
				"address": c.HQAddress,
				"err":     err.Error(),
			}).Errorln("error sending payload to crawl HQ, waiting 1s then retrying..")
			time.Sleep(time.Second)
			goto send
		}
	}
}

func (c *Crawl) hqConsumer() {
	crawlHQClient, err := gocrawlhq.Init(c.HQKey, c.HQSecret, c.HQProject, c.HQAddress)
	if err != nil {
		logrus.Panic(err)
	}

	for {
		if c.Finished.Get() {
			break
		}

		if c.Paused.Get() {
			time.Sleep(time.Second)
		}

		if c.ActiveWorkers.Value() >= int64(c.Workers-(c.Workers/10)) {
			time.Sleep(time.Millisecond * 100)
			continue
		}

		// get batch from crawl HQ
		batch, err := crawlHQClient.Feed(c.Workers)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"project": c.HQProject,
				"address": c.HQAddress,
				"err":     err.Error(),
			}).Errorln("error getting new URLs from crawl HQ")
		}

		// send all URLs received in the batch to the frontier
		for _, URL := range batch.URLs {
			newURL, err := url.Parse(URL.Value)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"project": c.HQProject,
					"address": c.HQAddress,
					"err":     err.Error(),
				}).Errorln("unable to parse URL received from crawl HQ, discarding")
			}

			c.Frontier.PushChan <- frontier.NewItem(newURL, nil, "seed", 0, URL.ID)
		}
	}
}
