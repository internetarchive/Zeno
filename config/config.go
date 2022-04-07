package config

import "github.com/urfave/cli/v2"

type Flags struct {
	UserAgent string
	Job       string
	Workers   int
	MaxHops   uint
	Headless  bool
	Seencheck bool
	JSON      bool
	Debug     bool

	DisabledHTMLTags      cli.StringSlice
	ExcludedHosts         cli.StringSlice
	DomainsCrawl          bool
	CaptureAlternatePages bool
	MaxRedirect           int
	MaxRetry              int

	Proxy       string
	BypassProxy cli.StringSlice

	CookieFile  string
	KeepCookies bool

	API              bool
	APIPort          string
	Prometheus       bool
	PrometheusPrefix string

	WARCPrefix   string
	WARCOperator string

	UseHQ      bool
	HQAddress  string
	HQProject  string
	HQKey      string
	HQSecret   string
	HQStrategy string
}

type Application struct {
	Flags Flags
}

var App *Application

func init() {
	App = &Application{}
}
