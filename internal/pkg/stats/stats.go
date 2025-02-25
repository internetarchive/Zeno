package stats

import (
	"sync"
	"sync/atomic"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
)

type stats struct {
	URLsCrawled           *rate
	SeedsFinished         *rate
	PreprocessorRoutines  *counter
	ArchiverRoutines      *counter
	PostprocessorRoutines *counter
	FinisherRoutines      *counter
	Paused                atomic.Bool
	HTTPReturnCodes       *rateBucket
	MeanHTTPResponseTime  *mean
	WARCWritingQueueSize  atomic.Int64
}

var (
	globalStats     *stats
	globalPromStats *prometheusStats
	doOnce          sync.Once
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
			FinisherRoutines:      &counter{},
			HTTPReturnCodes:       newRateBucket(),
			MeanHTTPResponseTime:  &mean{},
		}

		globalPromStats = &prometheusStats{
			urlCrawled:            prometheus.NewCounter(prometheus.CounterOpts{Name: config.Get().PrometheusPrefix + "url_crawled", Help: "Total number of URLs crawled"}),
			finishedSeeds:         prometheus.NewCounter(prometheus.CounterOpts{Name: config.Get().PrometheusPrefix + "finished_seeds", Help: "Total number of finished seeds"}),
			preprocessorRoutines:  prometheus.NewGauge(prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "preprocessor_routines", Help: "Number of preprocessor routines"}),
			archiverRoutines:      prometheus.NewGauge(prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "archiver_routines", Help: "Number of archiver routines"}),
			postprocessorRoutines: prometheus.NewGauge(prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "postprocessor_routines", Help: "Number of postprocessor routines"}),
			finisherRoutines:      prometheus.NewGauge(prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "finisher_routines", Help: "Number of finisher routines"}),
			paused:                prometheus.NewGauge(prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "paused", Help: "Is Zeno paused"}),
			http2xx:               prometheus.NewCounter(prometheus.CounterOpts{Name: config.Get().PrometheusPrefix + "http_2xx", Help: "Number of HTTP 2xx responses"}),
			http3xx:               prometheus.NewCounter(prometheus.CounterOpts{Name: config.Get().PrometheusPrefix + "http_3xx", Help: "Number of HTTP 3xx responses"}),
			http4xx:               prometheus.NewCounter(prometheus.CounterOpts{Name: config.Get().PrometheusPrefix + "http_4xx", Help: "Number of HTTP 4xx responses"}),
			http5xx:               prometheus.NewCounter(prometheus.CounterOpts{Name: config.Get().PrometheusPrefix + "http_5xx", Help: "Number of HTTP 5xx responses"}),
			meanHTTPRespTime:      prometheus.NewGauge(prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "mean_http_resp_time", Help: "Mean HTTP response time"}),
			warcWritingQueueSize:  prometheus.NewGauge(prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "warc_writing_queue_size", Help: "Size of the WARC writing queue"}),
		}

		if config.Get().Prometheus {
			registerPrometheusMetrics()
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
	globalStats.FinisherRoutines.reset()
	globalStats.HTTPReturnCodes.resetAll()
	globalStats.MeanHTTPResponseTime.reset()
}

// GetMapTUI returns a map of the current stats.
// This is used by the TUI to update the stats table.
func GetMapTUI() map[string]interface{} {
	return map[string]interface{}{
		"URL/s":                   globalStats.URLsCrawled.get(),
		"Total URL crawled":       globalStats.URLsCrawled.getTotal(),
		"Finished seeds":          globalStats.SeedsFinished.getTotal(),
		"Preprocessor routines":   globalStats.PreprocessorRoutines.get(),
		"Archiver routines":       globalStats.ArchiverRoutines.get(),
		"Postprocessor routines":  globalStats.PostprocessorRoutines.get(),
		"Finisher routines":       globalStats.FinisherRoutines.get(),
		"Is paused?":              globalStats.Paused.Load(),
		"HTTP 2xx/s":              bucketSum(globalStats.HTTPReturnCodes.getFiltered("2*")),
		"HTTP 3xx/s":              bucketSum(globalStats.HTTPReturnCodes.getFiltered("3*")),
		"HTTP 4xx/s":              bucketSum(globalStats.HTTPReturnCodes.getFiltered("4*")),
		"HTTP 5xx/s":              bucketSum(globalStats.HTTPReturnCodes.getFiltered("5*")),
		"Mean HTTP response time": globalStats.MeanHTTPResponseTime.get(),
		"WARC writing queue size": globalStats.WARCWritingQueueSize.Load(),
	}
}
