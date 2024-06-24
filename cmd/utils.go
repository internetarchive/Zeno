package cmd

import (
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/config"
	"github.com/internetarchive/Zeno/internal/pkg/crawl"
	"github.com/internetarchive/Zeno/internal/pkg/frontier"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/paulbellamy/ratecounter"
	"github.com/sirupsen/logrus"
)

// InitCrawlWithCMD takes a config.Flags struct and return a
// *crawl.Crawl initialized with it
func InitCrawlWithCMD(flags config.Flags) *crawl.Crawl {
	var c = new(crawl.Crawl)

	// Statistics counters
	c.CrawledSeeds = new(ratecounter.Counter)
	c.CrawledAssets = new(ratecounter.Counter)
	c.ActiveWorkers = new(ratecounter.Counter)
	c.URIsPerSecond = ratecounter.NewRateCounter(1 * time.Second)

	c.LiveStats = flags.LiveStats
	c.ElasticSearchURL = flags.ElasticSearchURL

	// Frontier
	c.Frontier = new(frontier.Frontier)

	// If the job name isn't specified, we generate a random name
	if flags.Job == "" {
		if flags.HQProject != "" {
			c.Job = flags.HQProject
		} else {
			UUID, err := uuid.NewUUID()
			if err != nil {
				logrus.Fatal(err)
			}

			c.Job = UUID.String()
		}
	} else {
		c.Job = flags.Job
	}

	c.JobPath = path.Join("jobs", flags.Job)

	c.Workers = flags.Workers
	c.WorkerPool = make([]*crawl.Worker, 0)
	c.WorkerStopTimeout = time.Second * 60 // Placeholder for WorkerStopTimeout
	c.MaxConcurrentAssets = flags.MaxConcurrentAssets

	c.Seencheck = flags.Seencheck
	c.HTTPTimeout = flags.HTTPTimeout
	c.MaxConcurrentRequestsPerDomain = flags.MaxConcurrentRequestsPerDomain
	c.RateLimitDelay = flags.RateLimitDelay
	c.CrawlTimeLimit = flags.CrawlTimeLimit

	// Defaults --max-crawl-time-limit to 10% more than --crawl-time-limit
	if flags.MaxCrawlTimeLimit == 0 && flags.CrawlTimeLimit != 0 {
		c.MaxCrawlTimeLimit = flags.CrawlTimeLimit + (flags.CrawlTimeLimit / 10)
	} else {
		c.MaxCrawlTimeLimit = flags.MaxCrawlTimeLimit
	}

	c.MaxRetry = flags.MaxRetry
	c.MaxRedirect = flags.MaxRedirect
	c.MaxHops = uint8(flags.MaxHops)
	c.DomainsCrawl = flags.DomainsCrawl
	c.DisableAssetsCapture = flags.DisableAssetsCapture
	c.DisabledHTMLTags = flags.DisabledHTMLTags.Value()
	c.ExcludedHosts = flags.ExcludedHosts.Value()
	c.IncludedHosts = flags.IncludedHosts.Value()
	c.CaptureAlternatePages = flags.CaptureAlternatePages
	c.ExcludedStrings = flags.ExcludedStrings.Value()

	// WARC settings
	c.WARCPrefix = flags.WARCPrefix
	c.WARCOperator = flags.WARCOperator

	if flags.WARCTempDir != "" {
		c.WARCTempDir = flags.WARCTempDir
	} else {
		c.WARCTempDir = path.Join(c.JobPath, "temp")
	}

	c.CDXDedupeServer = flags.CDXDedupeServer
	c.DisableLocalDedupe = flags.DisableLocalDedupe
	c.CertValidation = flags.CertValidation
	c.WARCFullOnDisk = flags.WARCFullOnDisk
	c.WARCPoolSize = flags.WARCPoolSize
	c.WARCDedupSize = flags.WARCDedupSize
	c.WARCCustomCookie = flags.WARCCustomCookie

	c.API = flags.API
	c.APIPort = flags.APIPort

	// If Prometheus is specified, then we make sure
	// c.API is true
	c.Prometheus = flags.Prometheus
	if c.Prometheus {
		c.API = true
		c.PrometheusMetrics = new(crawl.PrometheusMetrics)
		c.PrometheusMetrics.Prefix = flags.PrometheusPrefix
	}

	if flags.UserAgent != "Zeno" {
		c.UserAgent = flags.UserAgent
	} else {
		version := utils.GetVersion()
		c.UserAgent = "Mozilla/5.0 (compatible; archive.org_bot +http://archive.org/details/archive.org_bot) Zeno/" + version.Version[:7] + " warc/" + version.WarcVersion
	}
	c.Headless = flags.Headless

	c.CookieFile = flags.CookieFile
	c.KeepCookies = flags.KeepCookies

	// Proxy settings
	c.Proxy = flags.Proxy
	c.BypassProxy = flags.BypassProxy.Value()

	// Crawl HQ settings
	c.UseHQ = flags.UseHQ
	c.HQProject = flags.HQProject
	c.HQAddress = flags.HQAddress
	c.HQKey = flags.HQKey
	c.HQSecret = flags.HQSecret
	c.HQStrategy = flags.HQStrategy
	c.HQBatchSize = int(flags.HQBatchSize)
	c.HQContinuousPull = flags.HQContinuousPull
	c.HQRateLimitingSendBack = flags.HQRateLimitingSendBack

	return c
}
