package crawl

import (
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/sirupsen/logrus"
)

func (c *Crawl) logCrawlSuccess(executionStart time.Time, statusCode int, item *frontier.Item) {
	logInfo.WithFields(logrus.Fields{
		"queued":         c.Frontier.QueueCount.Value(),
		"crawled":        c.CrawledSeeds.Value() + c.CrawledAssets.Value(),
		"rate":           c.URIsPerSecond.Rate(),
		"status_code":    statusCode,
		"active_workers": c.ActiveWorkers.Value(),
		"hop":            item.Hop,
		"type":           item.Type,
		"execution_time": time.Since(executionStart),
	}).Info(utils.URLToString(item.URL))
}
