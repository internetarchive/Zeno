package stats

import "sync"

type stats struct {
	URLsCrawled   *rate
	SeedsFinished *rate
}

var (
	globalStats *stats
	doOnce      sync.Once
)

func Init() error {
	var done = false
	doOnce.Do(func() {
		globalStats = &stats{
			URLsCrawled:   &rate{},
			SeedsFinished: &rate{},
		}
		done = true
	})

	if !done {
		return ErrStatsAlreadyInitialized
	}
	return nil
}
