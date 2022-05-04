package crawl

import (
	"net/url"
	"strings"
	"time"

	"git.archive.org/wb/gocrawlhq"
	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/sirupsen/logrus"
)

func (c *Crawl) hqProducer() {
	for item := range c.HQProducerChannel {
	send:
		if c.Finished.Get() {
			break
		}

		discoveredURL := gocrawlhq.URL{
			Value: item.URL.String(),
			Via:   item.ParentItem.URL.String(),
		}

		for i := 0; uint8(i) < item.Hop; i++ {
			discoveredURL.Path += "L"
		}

		_, err := c.goCrawlHQClient.Discovered([]gocrawlhq.URL{discoveredURL}, item.Type, false)
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
		batch, err := c.goCrawlHQClient.Feed(c.Workers)
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

			c.Frontier.PushChan <- frontier.NewItem(newURL, nil, "seed", uint8(strings.Count(URL.Path, "L")), URL.ID)
		}
	}
}

func (c *Crawl) hqFinisher() {
	finishedArray := []gocrawlhq.URL{}
	for FinishURL := range c.HQFinishedChannel {
		if FinishURL.ID == "" {
			logrus.WithFields(logrus.Fields{
				"project": c.HQProject,
				"address": c.HQAddress,
				"url":     FinishURL.URL.String(),
			}).Infoln("URL has no ID, discarding")
			return
		}
		finishedArray = append(finishedArray, gocrawlhq.URL{ID: FinishURL.ID, Value: FinishURL.URL.String()})

		if len(finishedArray) == 50 {
		finish:
			_, err := c.goCrawlHQClient.Finished(finishedArray)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"project":       c.HQProject,
					"address":       c.HQAddress,
					"finishedArray": finishedArray,
					"err":           err.Error(),
				}).Errorln("error submitting finished urls to crawl HQ. retrying in one second...")
				time.Sleep(time.Second)
				goto finish
			}
			finishedArray = []gocrawlhq.URL{}
		}

	}
}
