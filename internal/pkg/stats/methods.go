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
		globalPromStats.urlCrawled.WithLabelValues(config.Get().JobPrometheus, hostname, version).Inc()
	}
}

/////////////////////////
//    SeedsFinished    //
/////////////////////////

// SeedsFinishedIncr increments the SeedsFinished counter by 1.
func SeedsFinishedIncr() {
	globalStats.SeedsFinished.incr(1)
	if globalPromStats != nil {
		globalPromStats.finishedSeeds.WithLabelValues(config.Get().JobPrometheus, hostname, version).Inc()
	}
}

//////////////////////////
// PreprocessorRoutines //
//////////////////////////

// PreprocessorRoutinesIncr increments the PreprocessorRoutines counter by 1.
func PreprocessorRoutinesIncr() {
	globalStats.PreprocessorRoutines.incr(1)
	if globalPromStats != nil {
		globalPromStats.preprocessorRoutines.WithLabelValues(config.Get().JobPrometheus, hostname, version).Inc()
	}
}

// PreprocessorRoutinesDecr decrements the PreprocessorRoutines counter by 1.
func PreprocessorRoutinesDecr() {
	globalStats.PreprocessorRoutines.decr(1)
	if globalPromStats != nil {
		globalPromStats.preprocessorRoutines.WithLabelValues(config.Get().JobPrometheus, hostname, version).Dec()
	}
}

//////////////////////////
//  ArchiverRoutines    //
//////////////////////////

// ArchiverRoutinesIncr increments the ArchiverRoutines counter by 1.
func ArchiverRoutinesIncr() {
	globalStats.ArchiverRoutines.incr(1)
	if globalPromStats != nil {
		globalPromStats.archiverRoutines.WithLabelValues(config.Get().JobPrometheus, hostname, version).Inc()
	}
}

// ArchiverRoutinesDecr decrements the ArchiverRoutines counter by 1.
func ArchiverRoutinesDecr() {
	globalStats.ArchiverRoutines.decr(1)
	if globalPromStats != nil {
		globalPromStats.archiverRoutines.WithLabelValues(config.Get().JobPrometheus, hostname, version).Dec()
	}
}

//////////////////////////
// PostprocessorRoutines //
//////////////////////////

// PostprocessorRoutinesIncr increments the PostprocessorRoutines counter by 1.
func PostprocessorRoutinesIncr() {
	globalStats.PostprocessorRoutines.incr(1)
	if globalPromStats != nil {
		globalPromStats.postprocessorRoutines.WithLabelValues(config.Get().JobPrometheus, hostname, version).Inc()
	}
}

// PostprocessorRoutinesDecr decrements the PostprocessorRoutines counter by 1.
func PostprocessorRoutinesDecr() {
	globalStats.PostprocessorRoutines.decr(1)
	if globalPromStats != nil {
		globalPromStats.postprocessorRoutines.WithLabelValues(config.Get().JobPrometheus, hostname, version).Dec()
	}
}

//////////////////////////
// FinisherRoutines //
//////////////////////////

// FinisherRoutinesIncr increments the FinisherRoutines counter by 1.
func FinisherRoutinesIncr() {
	globalStats.FinisherRoutines.incr(1)
	if globalPromStats != nil {
		globalPromStats.finisherRoutines.WithLabelValues(config.Get().JobPrometheus, hostname, version).Inc()
	}
}

// FinisherRoutinesDecr decrements the FinisherRoutines counter by 1.
func FinisherRoutinesDecr() {
	globalStats.FinisherRoutines.decr(1)
	if globalPromStats != nil {
		globalPromStats.finisherRoutines.WithLabelValues(config.Get().JobPrometheus, hostname, version).Dec()
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
			globalPromStats.paused.WithLabelValues(config.Get().JobPrometheus, hostname, version).Set(1)
		}
	}
}

// PausedUnset sets the Paused flag to false.
func PausedUnset() {
	swapped := globalStats.Paused.CompareAndSwap(true, false)
	if swapped {
		if globalPromStats != nil {
			globalPromStats.paused.WithLabelValues(config.Get().JobPrometheus, hostname, version).Set(0)
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
			globalPromStats.http2xx.WithLabelValues(config.Get().JobPrometheus, hostname, version).Inc()
		case strings.HasPrefix(key, "3"):
			globalPromStats.http3xx.WithLabelValues(config.Get().JobPrometheus, hostname, version).Inc()
		case strings.HasPrefix(key, "4"):
			globalPromStats.http4xx.WithLabelValues(config.Get().JobPrometheus, hostname, version).Inc()
		case strings.HasPrefix(key, "5"):
			globalPromStats.http5xx.WithLabelValues(config.Get().JobPrometheus, hostname, version).Inc()
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
		globalPromStats.warcWritingQueueSize.WithLabelValues(config.Get().JobPrometheus, hostname, version).Set(float64(value))
	}
}

//////////////////////////
//   MeanHTTPRespTime   //
//////////////////////////

// MeanHTTPRespTimeAdd adds the given value to the MeanHTTPRespTime.
func MeanHTTPRespTimeAdd(value time.Duration) {
	globalStats.MeanHTTPResponseTime.add(uint64(value.Milliseconds()))
	if globalPromStats != nil {
		globalPromStats.meanHTTPRespTime.WithLabelValues(config.Get().JobPrometheus, hostname, version).Observe(float64(value))
	}
}

//////////////////////////
// MeanProcessBodyTime  //
//////////////////////////

// MeanProcessBodyTimeAdd adds the given value to the MeanProcessBodyTime.
func MeanProcessBodyTimeAdd(value time.Duration) {
	globalStats.MeanProcessBodyTime.add(uint64(value.Milliseconds()))
	if globalPromStats != nil {
		globalPromStats.meanProcessBodyTime.WithLabelValues(config.Get().JobPrometheus, hostname, version).Observe(float64(value))
	}
}

////////////////////////////
// MeanWaitOnFeedbackTime //
////////////////////////////

// MeanWaitOnFeedbackTimeAdd adds the given value to the MeanWaitOnFeedbackTime.
func MeanWaitOnFeedbackTimeAdd(value time.Duration) {
	globalStats.MeanWaitOnFeedbackTime.add(uint64(value.Milliseconds()))
	if globalPromStats != nil {
		globalPromStats.meanWaitOnFeedbackTime.WithLabelValues(config.Get().JobPrometheus, hostname, version).Observe(float64(value))
	}
}

////////////////////////////
//   WARC Data Metrics    //
////////////////////////////

// WARCDataTotalBytesSet sets the WARC data total bytes metric.
func WARCDataTotalBytesSet(value int64) {
	globalStats.WARCDataTotalBytes.Store(value)
	if globalPromStats != nil {
		globalPromStats.dataTotalBytes.WithLabelValues(config.Get().JobPrometheus, hostname, version).Set(float64(value))
	}
}

func WARCDataTotalBytesContentLengthSet(value int64) {
	globalStats.WARCDataTotalBytesContentLength.Store(value)
	if globalPromStats != nil {
		globalPromStats.dataTotalBytesContentLength.WithLabelValues(config.Get().JobPrometheus, hostname, version).Set(float64(value))
	}
}

// WARCCDXDedupeTotalBytesSet sets the WARC CDX dedupe total bytes metric.
func WARCCDXDedupeTotalBytesSet(value int64) {
	globalStats.WARCCDXDedupeTotalBytes.Store(value)
	if globalPromStats != nil {
		globalPromStats.cdxDedupeTotalBytes.WithLabelValues(config.Get().JobPrometheus, hostname, version).Set(float64(value))
	}
}

// WARCDoppelgangerDedupeTotalBytesSet sets the WARC Doppelganger dedupe total bytes metric.
func WARCDoppelgangerDedupeTotalBytesSet(value int64) {
	globalStats.WARCDoppelgangerDedupeTotalBytes.Store(value)
	if globalPromStats != nil {
		globalPromStats.doppelgangerDedupeTotalBytes.WithLabelValues(config.Get().JobPrometheus, hostname, version).Set(float64(value))
	}
}

// WARCLocalDedupeTotalBytesSet sets the WARC local dedupe total bytes metric.
func WARCLocalDedupeTotalBytesSet(value int64) {
	globalStats.WARCLocalDedupeTotalBytes.Store(value)
	if globalPromStats != nil {
		globalPromStats.localDedupeTotalBytes.WithLabelValues(config.Get().JobPrometheus, hostname, version).Set(float64(value))
	}
}

// WARCCDXDedupeTotalSet sets the WARC CDX dedupe total count metric.
func WARCCDXDedupeTotalSet(value int64) {
	globalStats.WARCCDXDedupeTotal.Store(value)
	if globalPromStats != nil {
		globalPromStats.cdxDedupeTotal.WithLabelValues(config.Get().JobPrometheus, hostname, version).Set(float64(value))
	}
}

// WARCDoppelgangerDedupeTotalSet sets the WARC Doppelganger dedupe total count metric.
func WARCDoppelgangerDedupeTotalSet(value int64) {
	globalStats.WARCDoppelgangerDedupeTotal.Store(value)
	if globalPromStats != nil {
		globalPromStats.doppelgangerDedupeTotal.WithLabelValues(config.Get().JobPrometheus, hostname, version).Set(float64(value))
	}
}

// WARCLocalDedupeTotalSet sets the WARC local dedupe total count metric.
func WARCLocalDedupeTotalSet(value int64) {
	globalStats.WARCLocalDedupeTotal.Store(value)
	if globalPromStats != nil {
		globalPromStats.localDedupeTotal.WithLabelValues(config.Get().JobPrometheus, hostname, version).Set(float64(value))
	}
}
