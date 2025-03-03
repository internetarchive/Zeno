package stats

import (
	"strings"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/config"
)

/////////////////////////
//     URLsCrawled     //
/////////////////////////

// URLsCrawledIncr increments the URLsCrawled counter by 1.
func URLsCrawledIncr() {
	globalStats.URLsCrawled.incr(1)
	if globalPromStats != nil {
		globalPromStats.urlCrawled.WithLabelValues(config.Get().Job, hostname, version).Inc()
	}
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
	if globalPromStats != nil {
		globalPromStats.finishedSeeds.WithLabelValues(config.Get().Job, hostname, version).Inc()
	}
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
	if globalPromStats != nil {
		globalPromStats.preprocessorRoutines.WithLabelValues(config.Get().Job, hostname, version).Inc()
	}
}

// PreprocessorRoutinesDecr decrements the PreprocessorRoutines counter by 1.
func PreprocessorRoutinesDecr() {
	globalStats.PreprocessorRoutines.decr(1)
	if globalPromStats != nil {
		globalPromStats.preprocessorRoutines.WithLabelValues(config.Get().Job, hostname, version).Dec()
	}
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
	if globalPromStats != nil {
		globalPromStats.archiverRoutines.WithLabelValues(config.Get().Job, hostname, version).Inc()
	}
}

// ArchiverRoutinesDecr decrements the ArchiverRoutines counter by 1.
func ArchiverRoutinesDecr() {
	globalStats.ArchiverRoutines.decr(1)
	if globalPromStats != nil {
		globalPromStats.archiverRoutines.WithLabelValues(config.Get().Job, hostname, version).Dec()
	}
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
	if globalPromStats != nil {
		globalPromStats.postprocessorRoutines.WithLabelValues(config.Get().Job, hostname, version).Inc()
	}
}

// PostprocessorRoutinesDecr decrements the PostprocessorRoutines counter by 1.
func PostprocessorRoutinesDecr() {
	globalStats.PostprocessorRoutines.decr(1)
	if globalPromStats != nil {
		globalPromStats.postprocessorRoutines.WithLabelValues(config.Get().Job, hostname, version).Dec()
	}
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
	if globalPromStats != nil {
		globalPromStats.finisherRoutines.WithLabelValues(config.Get().Job, hostname, version).Inc()
	}
}

// FinisherRoutinesDecr decrements the FinisherRoutines counter by 1.
func FinisherRoutinesDecr() {
	globalStats.FinisherRoutines.decr(1)
	if globalPromStats != nil {
		globalPromStats.finisherRoutines.WithLabelValues(config.Get().Job, hostname, version).Dec()
	}
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
		if globalPromStats != nil {
			globalPromStats.paused.WithLabelValues(config.Get().Job, hostname, version).Set(1)
		}
	}
}

// PausedUnset sets the Paused flag to false.
func PausedUnset() {
	swapped := globalStats.Paused.CompareAndSwap(true, false)
	if swapped {
		if globalPromStats != nil {
			globalPromStats.paused.WithLabelValues(config.Get().Job, hostname, version).Set(0)
		}
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
	if globalPromStats != nil {
		switch {
		case strings.HasPrefix(key, "2"):
			globalPromStats.http2xx.WithLabelValues(config.Get().Job, hostname, version).Inc()
		case strings.HasPrefix(key, "3"):
			globalPromStats.http3xx.WithLabelValues(config.Get().Job, hostname, version).Inc()
		case strings.HasPrefix(key, "4"):
			globalPromStats.http4xx.WithLabelValues(config.Get().Job, hostname, version).Inc()
		case strings.HasPrefix(key, "5"):
			globalPromStats.http5xx.WithLabelValues(config.Get().Job, hostname, version).Inc()
		}
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
	if globalPromStats != nil {
		globalPromStats.warcWritingQueueSize.WithLabelValues(config.Get().Job, hostname, version).Set(float64(value))
	}
}

// WarcWritingQueueSizeGet returns the current value of the WarcWritingQueueSize.
func WarcWritingQueueSizeGet() int64 { return globalStats.WARCWritingQueueSize.Load() }

// WarcWritingQueueSizeReset resets the WarcWritingQueueSize to 0.
func WarcWritingQueueSizeReset() { globalStats.WARCWritingQueueSize.Store(0) }

//////////////////////////
//   MeanHTTPRespTime   //
//////////////////////////

// MeanHTTPRespTimeAdd adds the given value to the MeanHTTPRespTime.
func MeanHTTPRespTimeAdd(value time.Duration) {
	globalStats.MeanHTTPResponseTime.add(uint64(value.Milliseconds()))
	if globalPromStats != nil {
		globalPromStats.meanHTTPRespTime.WithLabelValues(config.Get().Job, hostname, version).Observe(float64(value))
	}
}

// MeanHTTPRespTimeGet returns the current value of the MeanHTTPRespTime.
func MeanHTTPRespTimeGet() float64 { return globalStats.MeanHTTPResponseTime.get() }

// MeanHTTPRespTimeReset resets the MeanHTTPRespTime to 0.
func MeanHTTPRespTimeReset() { globalStats.MeanHTTPResponseTime.reset() }

//////////////////////////
// MeanProcessBodyTime  //
//////////////////////////

// MeanProcessBodyTimeAdd adds the given value to the MeanProcessBodyTime.
func MeanProcessBodyTimeAdd(value time.Duration) {
	globalStats.MeanProcessBodyTime.add(uint64(value.Milliseconds()))
	if globalPromStats != nil {
		globalPromStats.meanProcessBodyTime.WithLabelValues(config.Get().Job, hostname, version).Observe(float64(value))
	}
}

// MeanProcessBodyTimeGet returns the current value of the MeanProcessBodyTime.
func MeanProcessBodyTimeGet() float64 { return globalStats.MeanProcessBodyTime.get() }

// MeanProcessBodyTimeReset resets the MeanProcessBodyTime to 0.
func MeanProcessBodyTimeReset() { globalStats.MeanProcessBodyTime.reset() }

////////////////////////////
// MeanWaitOnFeedbackTime //
////////////////////////////

// MeanWaitOnFeedbackTimeAdd adds the given value to the MeanWaitOnFeedbackTime.
func MeanWaitOnFeedbackTimeAdd(value time.Duration) {
	globalStats.MeanWaitOnFeedbackTime.add(uint64(value.Milliseconds()))
	if globalPromStats != nil {
		globalPromStats.meanWaitOnFeedbackTime.WithLabelValues(config.Get().Job, hostname, version).Observe(float64(value))
	}
}

// MeanWaitOnFeedbackTimeGet returns the current value of the MeanWaitOnFeedbackTime.
func MeanWaitOnFeedbackTimeGet() float64 { return globalStats.MeanWaitOnFeedbackTime.get() }

// MeanWaitOnFeedbackTimeReset resets the MeanWaitOnFeedbackTime to 0.
func MeanWaitOnFeedbackTimeReset() { globalStats.MeanWaitOnFeedbackTime.reset() }
