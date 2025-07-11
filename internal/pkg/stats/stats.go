package stats

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
)

type stats struct {
	URLsCrawled            *rate
	SeedsFinished          *rate
	PreprocessorRoutines   *counter
	ArchiverRoutines       *counter
	PostprocessorRoutines  *counter
	FinisherRoutines       *counter
	Paused                 atomic.Bool
	HTTPReturnCodes        *rateBucket
	MeanHTTPResponseTime   *mean // in ms
	MeanProcessBodyTime    *mean // in ms
	MeanWaitOnFeedbackTime *mean // in ms
	WARCWritingQueueSize   atomic.Int64

	WARCDataTotalBytes               atomic.Int64
	WARCDataTotalBytesContentLength  atomic.Int64
	WARCCDXDedupeTotalBytes          atomic.Int64
	WARCDoppelgangerDedupeTotalBytes atomic.Int64
	WARCLocalDedupeTotalBytes        atomic.Int64
	WARCCDXDedupeTotal               atomic.Int64
	WARCDoppelgangerDedupeTotal      atomic.Int64
	WARCLocalDedupeTotal             atomic.Int64
}

var (
	globalStats     *stats
	globalPromStats *prometheusStats
	doOnce          sync.Once
	hostname        string
	version         string
)

func Init() error {
	var done = false
	var err error

	doOnce.Do(func() {
		globalStats = &stats{
			URLsCrawled:            &rate{},
			SeedsFinished:          &rate{},
			PreprocessorRoutines:   &counter{},
			ArchiverRoutines:       &counter{},
			PostprocessorRoutines:  &counter{},
			FinisherRoutines:       &counter{},
			HTTPReturnCodes:        newRateBucket(),
			MeanHTTPResponseTime:   &mean{},
			MeanProcessBodyTime:    &mean{},
			MeanWaitOnFeedbackTime: &mean{},
		}

		if config.Get() != nil && config.Get().Prometheus {
			globalPromStats = newPrometheusStats()

			// Get the hostname via env or via command
			hostname, err = os.Hostname()
			if err != nil {
				return
			}

			// Get Zeno version
			versionStruct := utils.GetVersion()
			version = versionStruct.Version

			registerPrometheusMetrics()
		}

		done = true
	})

	if err != nil {
		return fmt.Errorf("error getting hostname: %w", err)
	}

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
	globalStats.MeanProcessBodyTime.reset()
	globalStats.MeanWaitOnFeedbackTime.reset()
}

// GetMapTUI returns a map of the current stats.
// This is used by the TUI to update the stats table.
func GetMapTUI() map[string]interface{} {
	result := map[string]interface{}{
		"URL/s":                      globalStats.URLsCrawled.get(),
		"Total URL crawled":          globalStats.URLsCrawled.getTotal(),
		"Finished seeds":             globalStats.SeedsFinished.getTotal(),
		"Preprocessor routines":      globalStats.PreprocessorRoutines.get(),
		"Archiver routines":          globalStats.ArchiverRoutines.get(),
		"Postprocessor routines":     globalStats.PostprocessorRoutines.get(),
		"Finisher routines":          globalStats.FinisherRoutines.get(),
		"Is paused?":                 globalStats.Paused.Load(),
		"HTTP 2xx/s":                 bucketSum(globalStats.HTTPReturnCodes.getFiltered("2*")),
		"HTTP 3xx/s":                 bucketSum(globalStats.HTTPReturnCodes.getFiltered("3*")),
		"HTTP 4xx/s":                 bucketSum(globalStats.HTTPReturnCodes.getFiltered("4*")),
		"HTTP 5xx/s":                 bucketSum(globalStats.HTTPReturnCodes.getFiltered("5*")),
		"Mean HTTP response time":    globalStats.MeanHTTPResponseTime.get(),
		"Mean wait on feedback time": globalStats.MeanWaitOnFeedbackTime.get(),
		"Mean process body time":     globalStats.MeanProcessBodyTime.get(),
		"WARC writing queue size":    globalStats.WARCWritingQueueSize.Load(),
		"WARC data total (GB)":       float64(globalStats.WARCDataTotalBytes.Load()) / 1e9,
	}

	// Only show CDX dedupe stats if activated and has data
	if config.Get().CDXDedupeServer != "" {
		if dedupeBytes := globalStats.WARCCDXDedupeTotalBytes.Load(); dedupeBytes > 0 {
			result["CDX dedupe bytes (GB)"] = float64(dedupeBytes) / 1e9
		}
		if dedupeCount := globalStats.WARCCDXDedupeTotal.Load(); dedupeCount > 0 {
			result["CDX dedupe count"] = dedupeCount
		}
	}

	// Only show Doppelganger dedupe stats if activated and has data
	if config.Get().DoppelgangerDedupeServer != "" {
		if dedupeBytes := globalStats.WARCDoppelgangerDedupeTotalBytes.Load(); dedupeBytes > 0 {
			result["Doppelganger dedupe bytes (GB)"] = float64(dedupeBytes) / 1e9
		}
		if dedupeCount := globalStats.WARCDoppelgangerDedupeTotal.Load(); dedupeCount > 0 {
			result["Doppelganger dedupe count"] = dedupeCount
		}
	}

	// Only show local dedupe stats if activated and has data
	if !config.Get().DisableLocalDedupe {
		if dedupeBytes := globalStats.WARCLocalDedupeTotalBytes.Load(); dedupeBytes > 0 {
			result["Local dedupe bytes (GB)"] = float64(dedupeBytes) / 1e9
		}
		if dedupeCount := globalStats.WARCLocalDedupeTotal.Load(); dedupeCount > 0 {
			result["Local dedupe count"] = dedupeCount
		}
	}

	return result
}
