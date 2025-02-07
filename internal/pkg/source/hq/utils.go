package hq

import (
	"strings"
	"time"
)

func pathToHops(path string) (hops int) {
	// For each L in the path, add 1 hop
	return strings.Count(path, "L")
}

func hopsToPath(hops int) (path string) {
	// For each hop, add an L to the path
	return strings.Repeat("L", hops)
}

// resetTimer safely resets the timer to the specified duration.
func resetTimer(timer *time.Timer, duration time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(duration)
}
