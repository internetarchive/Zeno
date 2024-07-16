package config

import "github.com/urfave/cli/v2"

type Flags struct {
	UserAgent           string
	Job                 string
	Workers             uint64
	MaxConcurrentAssets uint64
	MaxHops             uint64
	Headless            bool
	Seencheck           bool
	JSON                bool
	LiveStats           bool
	Debug               bool
	MinSpaceRequired    uint64

	DisabledHTMLTags               cli.StringSlice
	ExcludedHosts                  cli.StringSlice
	IncludedHosts                  cli.StringSlice
	DomainsCrawl                   bool
	CaptureAlternatePages          bool
	HTTPTimeout                    uint64
	MaxRedirect                    uint64
	MaxRetry                       uint64
	MaxConcurrentRequestsPerDomain uint64
	RateLimitDelay                 uint64
	CrawlTimeLimit                 uint64
	MaxCrawlTimeLimit              uint64
	RandomLocalIP                  bool

	Proxy       string
	BypassProxy cli.StringSlice

	CookieFile  string
	KeepCookies bool

	API              bool
	APIPort          string
	PrometheusPrefix string

	WARCPrefix       string
	WARCOperator     string
	WARCPoolSize     int
	WARCDedupSize    int
	WARCFullOnDisk   bool
	WARCTempDir      string
	WARCCustomCookie string

	UseHQ                  bool
	HQBatchSize            int64
	HQAddress              string
	HQProject              string
	HQKey                  string
	HQSecret               string
	HQStrategy             string
	HQContinuousPull       bool
	HQRateLimitingSendBack bool

	CDXDedupeServer      string
	DisableLocalDedupe   bool
	DisableAssetsCapture bool
	CertValidation       bool

	ElasticSearchURLs        string
	ElasticSearchUsername    string
	ElasticSearchPassword    string
	ElasticSearchIndexPrefix string
	ExcludedStrings          cli.StringSlice
	LogFileOutputDir         string
}

type Application struct {
	Flags Flags
}

var App *Application

func init() {
	App = &Application{}
}
