package stats

import (
	"sync"
	"sync/atomic"
)

type stats struct {
	URLsCrawled           *rate
	SeedsFinished         *rate
	PreprocessorRoutines  *counter
	ArchiverRoutines      *counter
	PostprocessorRoutines *counter
	FinisherRoutines      *counter
	Paused                atomic.Bool
	HTTPReturnCodes       *rateBucket
	MeanHTTPResponseTime  *mean
	WARCWritingQueueSize  atomic.Int64
}

var (
	globalStats *stats
	doOnce      sync.Once
)

func Init() error {
	var done = false

	doOnce.Do(func() {
		globalStats = &stats{
			URLsCrawled:           &rate{},
			SeedsFinished:         &rate{},
			PreprocessorRoutines:  &counter{},
			ArchiverRoutines:      &counter{},
			PostprocessorRoutines: &counter{},
			FinisherRoutines:      &counter{},
			HTTPReturnCodes:       newRateBucket(),
			MeanHTTPResponseTime:  &mean{},
		}
		done = true
	})

	if !done {
		return ErrStatsAlreadyInitialized
	}

	return nil
}

func Reset() {
	globalStats.URLsCrawled.reset()
	globalStats.SeedsFinished.reset()
	globalStats.PreprocessorRoutines.reset()
	globalStats.ArchiverRoutines.reset()
	globalStats.PostprocessorRoutines.reset()
	globalStats.FinisherRoutines.reset()
	globalStats.HTTPReturnCodes.resetAll()
	globalStats.MeanHTTPResponseTime.reset()
}

// GetMap returns a map of the current stats.
// This is used by the TUI to update the stats table.
func GetMap() map[string]interface{} {
	return map[string]interface{}{
		"URL/s":                   globalStats.URLsCrawled.get(),
		"Total URL crawled":       globalStats.URLsCrawled.getTotal(),
		"Finished seeds":          globalStats.SeedsFinished.getTotal(),
		"Preprocessor routines":   globalStats.PreprocessorRoutines.get(),
		"Archiver routines":       globalStats.ArchiverRoutines.get(),
		"Postprocessor routines":  globalStats.PostprocessorRoutines.get(),
		"Finisher routines":       globalStats.FinisherRoutines.get(),
		"Is paused?":              globalStats.Paused.Load(),
		"HTTP 2xx/s":              bucketSum(globalStats.HTTPReturnCodes.getFiltered("2*")),
		"HTTP 3xx/s":              bucketSum(globalStats.HTTPReturnCodes.getFiltered("3*")),
		"HTTP 4xx/s":              bucketSum(globalStats.HTTPReturnCodes.getFiltered("4*")),
		"HTTP 5xx/s":              bucketSum(globalStats.HTTPReturnCodes.getFiltered("5*")),
		"Mean HTTP response time": globalStats.MeanHTTPResponseTime.get(),
		"WARC writing queue size": globalStats.WARCWritingQueueSize.Load(),
	}
}
