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
	Paused                atomic.Bool
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
}

// GetMap returns a map of the current stats.
// This is used by the TUI to update the stats table.
func GetMap() map[string]interface{} {
	return map[string]interface{}{
		"URL/s":                  globalStats.URLsCrawled.get(),
		"Total URL crawled":      globalStats.URLsCrawled.getTotal(),
		"Finished seeds":         globalStats.SeedsFinished.get(),
		"Preprocessor routines":  globalStats.PreprocessorRoutines.get(),
		"Archiver routines":      globalStats.ArchiverRoutines.get(),
		"Postprocessor routines": globalStats.PostprocessorRoutines.get(),
		"Is paused?":             globalStats.Paused.Load(),
	}
}
