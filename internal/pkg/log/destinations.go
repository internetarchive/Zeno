// destination.go
package log

import (
	"log/slog"
)

// Destination interface
type Destination interface {
	Enabled() bool
	Level() slog.Level
	Write(entry *logEntry)
	Close()
}

func initDestinations() []Destination {
	var destinations []Destination

	if globalConfig.StdoutEnabled {
		destinations = append(destinations, &StdoutDestination{
			level: globalConfig.StdoutLevel,
		})
	}

	if globalConfig.StderrEnabled {
		destinations = append(destinations, &StderrDestination{
			level: globalConfig.StderrLevel,
		})
	}

	if globalConfig.FileConfig != nil {
		fileDest := NewFileDestination()
		destinations = append(destinations, fileDest)
	}

	if globalConfig.ElasticsearchConfig != nil {
		esDest := NewElasticsearchDestination()
		destinations = append(destinations, esDest)
	}

	if globalConfig.LogChanTUI {
		TUIDestination := NewTUIDestination()
		LogChanTUI = make(chan string, 10000)
		destinations = append(destinations, TUIDestination)
	}

	return destinations
}
