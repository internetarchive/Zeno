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
		globalPromStats.urlCrawled.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Inc()
	}
}

/////////////////////////
//    SeedsFinished    //
/////////////////////////

// SeedsFinishedIncr increments the SeedsFinished counter by 1.
func SeedsFinishedIncr() {
	globalStats.SeedsFinished.incr(1)
	if globalPromStats != nil {
		globalPromStats.finishedSeeds.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Inc()
	}
}

//////////////////////////
// PreprocessorRoutines //
//////////////////////////

// PreprocessorRoutinesIncr increments the PreprocessorRoutines counter by 1.
func PreprocessorRoutinesIncr() {
	globalStats.PreprocessorRoutines.incr(1)
	if globalPromStats != nil {
		globalPromStats.preprocessorRoutines.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Inc()
	}
}

// PreprocessorRoutinesDecr decrements the PreprocessorRoutines counter by 1.
func PreprocessorRoutinesDecr() {
	globalStats.PreprocessorRoutines.decr(1)
	if globalPromStats != nil {
		globalPromStats.preprocessorRoutines.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Dec()
	}
}

//////////////////////////
//  ArchiverRoutines    //
//////////////////////////

// ArchiverRoutinesIncr increments the ArchiverRoutines counter by 1.
func ArchiverRoutinesIncr() {
	globalStats.ArchiverRoutines.incr(1)
	if globalPromStats != nil {
		globalPromStats.archiverRoutines.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Inc()
	}
}

// ArchiverRoutinesDecr decrements the ArchiverRoutines counter by 1.
func ArchiverRoutinesDecr() {
	globalStats.ArchiverRoutines.decr(1)
	if globalPromStats != nil {
		globalPromStats.archiverRoutines.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Dec()
	}
}

//////////////////////////
// PostprocessorRoutines //
//////////////////////////

// PostprocessorRoutinesIncr increments the PostprocessorRoutines counter by 1.
func PostprocessorRoutinesIncr() {
	globalStats.PostprocessorRoutines.incr(1)
	if globalPromStats != nil {
		globalPromStats.postprocessorRoutines.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Inc()
	}
}

// PostprocessorRoutinesDecr decrements the PostprocessorRoutines counter by 1.
func PostprocessorRoutinesDecr() {
	globalStats.PostprocessorRoutines.decr(1)
	if globalPromStats != nil {
		globalPromStats.postprocessorRoutines.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Dec()
	}
}

//////////////////////////
// FinisherRoutines //
//////////////////////////

// FinisherRoutinesIncr increments the FinisherRoutines counter by 1.
func FinisherRoutinesIncr() {
	globalStats.FinisherRoutines.incr(1)
	if globalPromStats != nil {
		globalPromStats.finisherRoutines.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Inc()
	}
}

// FinisherRoutinesDecr decrements the FinisherRoutines counter by 1.
func FinisherRoutinesDecr() {
	globalStats.FinisherRoutines.decr(1)
	if globalPromStats != nil {
		globalPromStats.finisherRoutines.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Dec()
	}
}

//////////////////////////
//         Paused       //
//////////////////////////

// PausedSet sets the Paused flag to true.
func PausedSet() {
	swapped := globalStats.Paused.CompareAndSwap(false, true)
	if swapped {
		if globalPromStats != nil {
			globalPromStats.paused.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Set(1)
		}
	}
}

// PausedUnset sets the Paused flag to false.
func PausedUnset() {
	swapped := globalStats.Paused.CompareAndSwap(true, false)
	if swapped {
		if globalPromStats != nil {
			globalPromStats.paused.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Set(0)
		}
	}
}

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
			globalPromStats.http2xx.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Inc()
		case strings.HasPrefix(key, "3"):
			globalPromStats.http3xx.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Inc()
		case strings.HasPrefix(key, "4"):
			globalPromStats.http4xx.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Inc()
		case strings.HasPrefix(key, "5"):
			globalPromStats.http5xx.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Inc()
		}
	}
}

//////////////////////////
// WarcWritingQueueSize //
//////////////////////////

// WarcWritingQueueSizeSet sets the WarcWritingQueueSize to the given value.
func WarcWritingQueueSizeSet(value int64) {
	globalStats.WARCWritingQueueSize.Store(value)
	if globalPromStats != nil {
		globalPromStats.warcWritingQueueSize.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Set(float64(value))
	}
}

//////////////////////////
//   MeanHTTPRespTime   //
//////////////////////////

// MeanHTTPRespTimeAdd adds the given value to the MeanHTTPRespTime.
func MeanHTTPRespTimeAdd(value time.Duration) {
	globalStats.MeanHTTPResponseTime.add(uint64(value.Milliseconds()))
	if globalPromStats != nil {
		globalPromStats.meanHTTPRespTime.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Observe(float64(value))
	}
}

//////////////////////////
// MeanProcessBodyTime  //
//////////////////////////

// MeanProcessBodyTimeAdd adds the given value to the MeanProcessBodyTime.
func MeanProcessBodyTimeAdd(value time.Duration) {
	globalStats.MeanProcessBodyTime.add(uint64(value.Milliseconds()))
	if globalPromStats != nil {
		globalPromStats.meanProcessBodyTime.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Observe(float64(value))
	}
}

////////////////////////////
// MeanWaitOnFeedbackTime //
////////////////////////////

// MeanWaitOnFeedbackTimeAdd adds the given value to the MeanWaitOnFeedbackTime.
func MeanWaitOnFeedbackTimeAdd(value time.Duration) {
	globalStats.MeanWaitOnFeedbackTime.add(uint64(value.Milliseconds()))
	if globalPromStats != nil {
		globalPromStats.meanWaitOnFeedbackTime.WithLabelValues(strings.ReplaceAll(config.Get().Job, "-", ""), hostname, version).Observe(float64(value))
	}
}
