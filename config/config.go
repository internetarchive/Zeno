package config

import "github.com/urfave/cli/v2"

type Flags struct {
	Proxy     string
	UserAgent string
	Job       string
	Workers   int
	MaxHops   uint
	Headless  bool
	Seencheck bool
	JSON      bool
	Debug     bool

	DomainsCrawl bool

	API bool

	WARC         bool
	WARCRetry    int
	WARCPrefix   string
	WARCOperator string

	Kafka              bool
	KafkaFeedTopic     string
	KafkaOutlinksTopic string
	KafkaConsumerGroup string
	KafkaBrokers       cli.StringSlice
}

type Application struct {
	Flags Flags
}

var App *Application

func init() {
	App = &Application{}
}
