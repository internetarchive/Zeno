package crawl

import (
	"math"
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

		_, err := c.HQClient.Discovered([]gocrawlhq.URL{discoveredURL}, item.Type, false, false)
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
		batch, err := c.HQClient.Feed(int(math.Ceil(float64(c.Workers)/2)), c.HQStrategy)
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

	for finishedURL := range c.HQFinishedChannel {
		if finishedURL.ID == "" {
			logrus.WithFields(logrus.Fields{
				"project": c.HQProject,
				"address": c.HQAddress,
				"url":     finishedURL.URL.String(),
			}).Infoln("URL has no ID, discarding")
			continue
		}

		finishedArray = append(finishedArray, gocrawlhq.URL{ID: finishedURL.ID, Value: finishedURL.URL.String()})

		if len(finishedArray) == int(math.Ceil(float64(c.Workers)/2)) {
		finish:
			_, err := c.HQClient.Finished(finishedArray)
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

func (c *Crawl) hqSeencheck(URL *url.URL) (bool, error) {
	discoveredURL := gocrawlhq.URL{
		Value: URL.String(),
	}

	discoveredResponse, err := c.HQClient.Discovered([]gocrawlhq.URL{discoveredURL}, "asset", false, true)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"project": c.HQProject,
			"address": c.HQAddress,
			"url":     URL.String(),
			"err":     err.Error(),
		}).Errorln("error sending seencheck payload to crawl HQ")
		return false, err
	}

	if discoveredResponse.URLs != nil {
		for _, URL := range discoveredResponse.URLs {
			if URL.Value == discoveredURL.Value {
				return false, nil
			}
		}
	}

	// didn't find the URL in the HQ, so it's new and has been added to HQ's seencheck database
	return true, nil
}
