package crawl

import (
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/sirupsen/logrus"
)

func (c *Crawl) logCrawlSuccess(executionStart time.Time, statusCode int, item *frontier.Item) {
	logInfo.WithFields(logrus.Fields{
		"queued":        c.Frontier.QueueCount.Value(),
		"crawled":       c.CrawledSeeds.Value() + c.CrawledAssets.Value(),
		"rate":          c.URIsPerSecond.Rate(),
		"statusCode":    statusCode,
		"activeWorkers": c.ActiveWorkers.Value(),
		"hop":           item.Hop,
		"type":          item.Type,
		"executionTime": time.Since(executionStart).Milliseconds(),
		"url":           utils.URLToString(item.URL),
	}).Info("URL archived")
}
