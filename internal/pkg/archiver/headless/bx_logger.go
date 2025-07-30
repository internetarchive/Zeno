package headless

import (
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/ysmood/gson"
)

// ZenoBxLogger is a custom logger for Browsertrix behaviors.
// https://github.com/webrecorder/browsertrix-behaviors/#logging
type ZenoBxLogger struct {
	item               *models.Item
	lastMessage        string
	suppressedMessages int
}

func newBxLogger(item *models.Item) *ZenoBxLogger {
	return &ZenoBxLogger{
		item: item,
	}
}

// LogFunc expose itself to browser, so we can use our logger function (Golang) as browsertrix-behaviors' log function (JS).
func (l *ZenoBxLogger) LogFunc(v gson.JSON) (any, error) {
	var logger = log.NewFieldedLogger(&log.Fields{
		"component": "archiver.headless.bx_logger",
	})

	vMap := v.Map()
	loggerFunc := logger.Info
	args := make([]any, 0, 6+len(vMap)*2) // 6 for the "item_id", "url", "suppressed" and their values
	args = append(args, "item_id", l.item.GetShortID(), "url", l.item.GetURL().String())
	if logLevel, ok := vMap["type"]; ok {
		switch logLevel.String() {
		case "debug":
			loggerFunc = logger.Debug
		case "error":
			loggerFunc = logger.Error
		case "info":
			loggerFunc = logger.Info
		}
		delete(vMap, "type")
	}

	if dataMessage, ok := vMap["data"]; ok {
		if l.lastMessage == dataMessage.String() {
			// Suppress spamming the same scroll message
			l.suppressedMessages += 1
			return nil, nil
		}
		l.lastMessage = dataMessage.String()
	}

	for k, val := range vMap {
		args = append(args, k, val)
	}

	if l.suppressedMessages > 0 {
		args = append(args, "suppressed", l.suppressedMessages)
		l.suppressedMessages = 0
	}

	loggerFunc("page log", args...)
	return nil, nil
}
