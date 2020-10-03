package config

import "github.com/urfave/cli/v2"

type Flags struct {
	Proxy     string
	UserAgent string
	Job       string
	Workers   int
	MaxHops   uint
	WARC      bool
	Headless  bool
	Seencheck bool
	JSON      bool
	Debug     bool

	Kafka              bool
	KafkaFeedTopic     string
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
