package stats

import (
	"strings"
)

/////////////////////////
//     URLsCrawled     //
/////////////////////////

// URLsCrawledIncr increments the URLsCrawled counter by 1.
func URLsCrawledIncr() {
	globalStats.URLsCrawled.incr(1)
	globalPromStats.urlCrawled.Inc()
}

// URLsCrawledGet returns the current value of the URLsCrawled counter.
func URLsCrawledGet() uint64 { return globalStats.URLsCrawled.get() }

// URLsCrawledReset resets the URLsCrawled counter to 0.
func URLsCrawledReset() { globalStats.URLsCrawled.reset() }

/////////////////////////
//    SeedsFinished    //
/////////////////////////

// SeedsFinishedIncr increments the SeedsFinished counter by 1.
func SeedsFinishedIncr() {
	globalStats.SeedsFinished.incr(1)
	globalPromStats.finishedSeeds.Inc()
}

// SeedsFinishedGet returns the current value of the SeedsFinished counter.
func SeedsFinishedGet() uint64 { return globalStats.SeedsFinished.get() }

// SeedsFinishedReset resets the SeedsFinished counter to 0.
func SeedsFinishedReset() { globalStats.SeedsFinished.reset() }

//////////////////////////
// PreprocessorRoutines //
//////////////////////////

// PreprocessorRoutinesIncr increments the PreprocessorRoutines counter by 1.
func PreprocessorRoutinesIncr() {
	globalStats.PreprocessorRoutines.incr(1)
	globalPromStats.preprocessorRoutines.Inc()
}

// PreprocessorRoutinesDecr decrements the PreprocessorRoutines counter by 1.
func PreprocessorRoutinesDecr() {
	globalStats.PreprocessorRoutines.decr(1)
	globalPromStats.preprocessorRoutines.Dec()
}

// PreprocessorRoutinesGet returns the current value of the PreprocessorRoutines counter.
func PreprocessorRoutinesGet() uint64 { return globalStats.PreprocessorRoutines.get() }

// PreprocessorRoutinesReset resets the PreprocessorRoutines counter to 0.
func PreprocessorRoutinesReset() { globalStats.PreprocessorRoutines.reset() }

//////////////////////////
//  ArchiverRoutines    //
//////////////////////////

// ArchiverRoutinesIncr increments the ArchiverRoutines counter by 1.
func ArchiverRoutinesIncr() {
	globalStats.ArchiverRoutines.incr(1)
	globalPromStats.archiverRoutines.Inc()
}

// ArchiverRoutinesDecr decrements the ArchiverRoutines counter by 1.
func ArchiverRoutinesDecr() {
	globalStats.ArchiverRoutines.decr(1)
	globalPromStats.archiverRoutines.Dec()
}

// ArchiverRoutinesGet returns the current value of the ArchiverRoutines counter.
func ArchiverRoutinesGet() uint64 { return globalStats.ArchiverRoutines.get() }

// ArchiverRoutinesReset resets the ArchiverRoutines counter to 0.
func ArchiverRoutinesReset() { globalStats.ArchiverRoutines.reset() }

//////////////////////////
// PostprocessorRoutines //
//////////////////////////

// PostprocessorRoutinesIncr increments the PostprocessorRoutines counter by 1.
func PostprocessorRoutinesIncr() {
	globalStats.PostprocessorRoutines.incr(1)
	globalPromStats.postprocessorRoutines.Inc()
}

// PostprocessorRoutinesDecr decrements the PostprocessorRoutines counter by 1.
func PostprocessorRoutinesDecr() {
	globalStats.PostprocessorRoutines.decr(1)
	globalPromStats.postprocessorRoutines.Dec()
}

// PostprocessorRoutinesGet returns the current value of the PostprocessorRoutines counter.
func PostprocessorRoutinesGet() uint64 { return globalStats.PostprocessorRoutines.get() }

// PostprocessorRoutinesReset resets the PostprocessorRoutines counter to 0.
func PostprocessorRoutinesReset() { globalStats.PostprocessorRoutines.reset() }

//////////////////////////
// FinisherRoutines //
//////////////////////////

// FinisherRoutinesIncr increments the FinisherRoutines counter by 1.
func FinisherRoutinesIncr() {
	globalStats.FinisherRoutines.incr(1)
	globalPromStats.finisherRoutines.Inc()
}

// FinisherRoutinesDecr decrements the FinisherRoutines counter by 1.
func FinisherRoutinesDecr() {
	globalStats.FinisherRoutines.decr(1)
	globalPromStats.finisherRoutines.Dec()
}

// FinisherRoutinesGet returns the current value of the FinisherRoutines counter.
func FinisherRoutinesGet() uint64 { return globalStats.FinisherRoutines.get() }

// FinisherRoutinesReset resets the FinisherRoutines counter to 0.
func FinisherRoutinesReset() { globalStats.FinisherRoutines.reset() }

//////////////////////////
//         Paused       //
//////////////////////////

// PausedSet sets the Paused flag to true.
func PausedSet() {
	swapped := globalStats.Paused.CompareAndSwap(false, true)
	if swapped {
		globalPromStats.paused.Set(1)
	}
}

// PausedUnset sets the Paused flag to false.
func PausedUnset() {
	swapped := globalStats.Paused.CompareAndSwap(true, false)
	if swapped {
		globalPromStats.paused.Set(0)
	}
}

// PausedGet returns the current value of the Paused flag.
func PausedGet() bool { return globalStats.Paused.Load() }

// PausedReset resets the Paused flag to false.
func PausedReset() { globalStats.Paused.Store(false) }

//////////////////////////
//   HTTPReturnCodes    //
//////////////////////////

// HTTPReturnCodesIncr increments the HTTPReturnCodes counter for the given key by 1.
func HTTPReturnCodesIncr(key string) {
	globalStats.HTTPReturnCodes.incr(key, 1)
	switch {
	case strings.HasPrefix(key, "2"):
		globalPromStats.http2xx.Inc()
	case strings.HasPrefix(key, "3"):
		globalPromStats.http3xx.Inc()
	case strings.HasPrefix(key, "4"):
		globalPromStats.http4xx.Inc()
	case strings.HasPrefix(key, "5"):
		globalPromStats.http5xx.Inc()
	}
}

// HTTPReturnCodesGet returns the current value of the HTTPReturnCodes counter for the given key.
func HTTPReturnCodesGet(key string) uint64 { return globalStats.HTTPReturnCodes.get(key) }

// HTTPReturnCodesReset resets the HTTPReturnCodes counter for the given key to 0.
func HTTPReturnCodesReset(key string) { globalStats.HTTPReturnCodes.reset(key) }

// HTTPReturnCodesResetAll resets all HTTPReturnCodes counters to 0.
func HTTPReturnCodesResetAll() { globalStats.HTTPReturnCodes.resetAll() }

//////////////////////////
// WarcWritingQueueSize //
//////////////////////////

// WarcWritingQueueSizeSet sets the WarcWritingQueueSize to the given value.
func WarcWritingQueueSizeSet(value int64) {
	globalStats.WARCWritingQueueSize.Store(value)
	globalPromStats.warcWritingQueueSize.Set(float64(value))
}

// WarcWritingQueueSizeGet returns the current value of the WarcWritingQueueSize.
func WarcWritingQueueSizeGet() int64 { return globalStats.WARCWritingQueueSize.Load() }
