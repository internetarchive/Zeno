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
