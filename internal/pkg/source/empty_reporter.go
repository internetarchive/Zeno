package source

import (
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/log"
)

// FeedEmptyReporter logs when the feed is empty for a long time.
type FeedEmptyReporter struct {
	attempts   int
	start      time.Time
	lastReport time.Time
	interval   time.Duration
	logger     *log.FieldedLogger
}

func NewFeedEmptyReporter(logger *log.FieldedLogger) *FeedEmptyReporter {
	return &FeedEmptyReporter{
		logger:   logger,
		interval: time.Duration(10 * time.Second),
	}
}

func (e *FeedEmptyReporter) Report(urls int) {
	if urls == 0 {
		if e.attempts == 0 {
			e.start = time.Now()
			e.lastReport = time.Now()
		}
		e.attempts++
		if time.Since(e.lastReport) >= e.interval {
			e.logger.Info("feed is empty, waiting for new URLs",
				"attempts", e.attempts,
				"duration", time.Since(e.start),
			)
			e.lastReport = time.Now()
			e.interval *= 2
			if e.interval > 10*time.Minute {
				e.interval = 10 * time.Minute
			}
		}
	} else {
		e.attempts = 0
		e.start = time.Time{}
		e.lastReport = time.Time{}
	}
}
