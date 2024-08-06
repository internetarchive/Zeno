package stats

import "errors"

var (
	// ErrStatsNotInitialized is returned when the stats package is not properly initialized
	ErrStatsNotInitialized = errors.New("stats package not initialized")
)
