package crawl

import (
	"net/url"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/CorentinB/warc"
	"github.com/sirupsen/logrus"
)

func (c *Crawl) genLogFields(err interface{}, URL interface{}, additionalFields map[string]interface{}) (fields logrus.Fields) {
	fields = logrus.Fields{}

	fields["queued"] = c.Frontier.QueueCount.Value()
	fields["crawled"] = c.CrawledSeeds.Value() + c.CrawledAssets.Value()
	fields["rate"] = c.URIsPerSecond.Rate()
	fields["activeWorkers"] = c.ActiveWorkers.Value()

	if c.HQProject != "" {
		fields["hqProject"] = c.HQProject
		fields["hqAddress"] = c.HQAddress
	}

	if c.Job != "" {
		fields["job"] = c.Job
	}

	switch errValue := err.(type) {
	case error:
		fields["err"] = errValue.Error()
	case *warc.Error:
		fields["err"] = errValue.Err.Error()
		fields["errFunc"] = errValue.Func
	default:
	}

	switch URLValue := URL.(type) {
	case string:
		fields["url"] = URLValue
	case *url.URL:
		fields["url"] = utils.URLToString(URLValue)
	case url.URL:
		fields["url"] = utils.URLToString(&URLValue)
	default:
	}

	for key, value := range additionalFields {
		fields[key] = value
	}

	return fields
}

func (c *Crawl) logCrawlSuccess(executionStart time.Time, statusCode int, item *frontier.Item) {
	fields := c.genLogFields(nil, item.URL, nil)

	fields["statusCode"] = statusCode
	fields["hop"] = item.Hop
	fields["type"] = item.Type
	fields["executionTime"] = time.Since(executionStart).Milliseconds()
	fields["url"] = utils.URLToString(item.URL)

	logInfo.WithFields(fields).Info("URL archived")
}
