package config

import "github.com/urfave/cli/v2"

type Flags struct {
	UserAgent           string
	Job                 string
	Workers             int
	MaxConcurrentAssets int
	MaxHops             uint
	Seencheck           bool
	JSON                bool
	LiveStats           bool
	Debug               bool

	DisabledHTMLTags               cli.StringSlice
	ExcludedHosts                  cli.StringSlice
	DomainsCrawl                   bool
	CaptureAlternatePages          bool
	HTTPTimeout                    int
	MaxRedirect                    int
	MaxRetry                       int
	MaxConcurrentRequestsPerDomain int
	RateLimitDelay                 int

	Proxy       string
	BypassProxy cli.StringSlice

	CookieFile  string
	KeepCookies bool

	API              bool
	APIPort          string
	Prometheus       bool
	PrometheusPrefix string

	WARCPrefix     string
	WARCOperator   string
	WARCPoolSize   int
	WARCDedupSize  int
	WARCFullOnDisk bool
	WARCTempDir    string

	UseHQ            bool
	HQBatchSize      int64
	HQAddress        string
	HQProject        string
	HQKey            string
	HQSecret         string
	HQStrategy       string
	HQContinuousPull bool

	CDXDedupeServer      string
	DisableLocalDedupe   bool
	DisableAssetsCapture bool
	CertValidation       bool

	Headless              bool
	Headfull              bool
	HeadlessWaitAfterLoad uint64

	Cloudflarestream bool
	ElasticSearchURL string
	ExcludedStrings  cli.StringSlice
}

type Application struct {
	Flags Flags
}

var App *Application

func init() {
	App = &Application{}
}
