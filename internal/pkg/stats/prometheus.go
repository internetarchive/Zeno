package stats

import (
	"net/http"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type prometheusStats struct {
	urlCrawled             *prometheus.CounterVec
	finishedSeeds          *prometheus.CounterVec
	preprocessorRoutines   *prometheus.GaugeVec
	archiverRoutines       *prometheus.GaugeVec
	postprocessorRoutines  *prometheus.GaugeVec
	finisherRoutines       *prometheus.GaugeVec
	paused                 *prometheus.GaugeVec
	http2xx                *prometheus.CounterVec
	http3xx                *prometheus.CounterVec
	http4xx                *prometheus.CounterVec
	http5xx                *prometheus.CounterVec
	meanHTTPRespTime       *prometheus.HistogramVec // in ns
	meanProcessBodyTime    *prometheus.HistogramVec // in ns
	meanWaitOnFeedbackTime *prometheus.HistogramVec // in ns
	warcWritingQueueSize   *prometheus.GaugeVec

	// Dedup WARC metrics
	dataTotalBytes               *prometheus.GaugeVec
	dataTotalBytesContentLength  *prometheus.GaugeVec
	cdxDedupeTotalBytes          *prometheus.GaugeVec
	doppelgangerDedupeTotalBytes *prometheus.GaugeVec
	localDedupeTotalBytes        *prometheus.GaugeVec
	cdxDedupeTotal               *prometheus.GaugeVec
	doppelgangerDedupeTotal      *prometheus.GaugeVec
	localDedupeTotal             *prometheus.GaugeVec
}

func newPrometheusStats() *prometheusStats {
	return &prometheusStats{
		urlCrawled: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: config.Get().PrometheusPrefix + "url_crawled", Help: "Total number of URLs crawled"},
			[]string{"project", "hostname", "version"},
		),
		finishedSeeds: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: config.Get().PrometheusPrefix + "finished_seeds", Help: "Total number of finished seeds"},
			[]string{"project", "hostname", "version"},
		),
		preprocessorRoutines: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "preprocessor_routines", Help: "Number of preprocessor routines"},
			[]string{"project", "hostname", "version"},
		),
		archiverRoutines: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "archiver_routines", Help: "Number of archiver routines"},
			[]string{"project", "hostname", "version"},
		),
		postprocessorRoutines: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "postprocessor_routines", Help: "Number of postprocessor routines"},
			[]string{"project", "hostname", "version"},
		),
		finisherRoutines: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "finisher_routines", Help: "Number of finisher routines"},
			[]string{"project", "hostname", "version"},
		),
		paused: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "paused", Help: "Is Zeno paused"},
			[]string{"project", "hostname", "version"},
		),
		http2xx: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: config.Get().PrometheusPrefix + "http_2xx", Help: "Number of HTTP 2xx responses"},
			[]string{"project", "hostname", "version"},
		),
		http3xx: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: config.Get().PrometheusPrefix + "http_3xx", Help: "Number of HTTP 3xx responses"},
			[]string{"project", "hostname", "version"},
		),
		http4xx: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: config.Get().PrometheusPrefix + "http_4xx", Help: "Number of HTTP 4xx responses"},
			[]string{"project", "hostname", "version"},
		),
		http5xx: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: config.Get().PrometheusPrefix + "http_5xx", Help: "Number of HTTP 5xx responses"},
			[]string{"project", "hostname", "version"},
		),
		meanHTTPRespTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{Name: config.Get().PrometheusPrefix + "mean_http_resp_time", Help: "Mean HTTP response time in ns", Buckets: prometheus.ExponentialBucketsRange(float64(20*time.Millisecond), float64(10*time.Second), 50)},
			[]string{"project", "hostname", "version"},
		),
		meanProcessBodyTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{Name: config.Get().PrometheusPrefix + "mean_process_body_time", Help: "Mean time in ns to process the body of a response", Buckets: prometheus.ExponentialBucketsRange(float64(time.Microsecond), float64(10*time.Second), 50)},
			[]string{"project", "hostname", "version"},
		),
		meanWaitOnFeedbackTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{Name: config.Get().PrometheusPrefix + "mean_wait_on_feedback_time", Help: "Mean time in ns to wait on WARC writing feedback signal", Buckets: prometheus.ExponentialBucketsRange(float64(time.Microsecond), float64(10*time.Second), 50)},
			[]string{"project", "hostname", "version"},
		),
		warcWritingQueueSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "warc_writing_queue_size", Help: "Size of the WARC writing queue"},
			[]string{"project", "hostname", "version"},
		),

		// Dedup WARC metrics
		dataTotalBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "total_bytes_downloaded", Help: "Total number of bytes downloaded through gowarc"},
			[]string{"project", "hostname", "version"},
		),
		// Potentially temporary "Content-Length" bytes downloaded metric
		dataTotalBytesContentLength: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "total_bytes_downloaded_content_length", Help: "Total number of bytes downloaded through gowarc as measured by Content-Length header"},
			[]string{"project", "hostname", "version"},
		),
		cdxDedupeTotalBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "total_bytes_cdx_dedupe", Help: "Total number of bytes deduplicated by CDX"},
			[]string{"project", "hostname", "version"},
		),
		doppelgangerDedupeTotalBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "total_bytes_doppelganger_dedupe", Help: "Total number of bytes deduplicated by Doppelganger"},
			[]string{"project", "hostname", "version"},
		),
		localDedupeTotalBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "total_bytes_local_dedupe", Help: "Total number of bytes deduplicated by local hash table"},
			[]string{"project", "hostname", "version"},
		),
		cdxDedupeTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "total_cdx_dedupe", Help: "Total number of successful CDX hits"},
			[]string{"project", "hostname", "version"},
		),
		doppelgangerDedupeTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "total_doppelganger_dedupe", Help: "Total number of succcessful Doppelganger hits"},
			[]string{"project", "hostname", "version"},
		),
		localDedupeTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "total_local_dedupe", Help: "Total number of local hash table hits"},
			[]string{"project", "hostname", "version"},
		),
	}
}

func registerPrometheusMetrics() {
	prometheus.MustRegister(globalPromStats.urlCrawled)
	prometheus.MustRegister(globalPromStats.finishedSeeds)
	prometheus.MustRegister(globalPromStats.preprocessorRoutines)
	prometheus.MustRegister(globalPromStats.archiverRoutines)
	prometheus.MustRegister(globalPromStats.postprocessorRoutines)
	prometheus.MustRegister(globalPromStats.finisherRoutines)
	prometheus.MustRegister(globalPromStats.paused)
	prometheus.MustRegister(globalPromStats.http2xx)
	prometheus.MustRegister(globalPromStats.http3xx)
	prometheus.MustRegister(globalPromStats.http4xx)
	prometheus.MustRegister(globalPromStats.http5xx)
	prometheus.MustRegister(globalPromStats.meanHTTPRespTime)
	prometheus.MustRegister(globalPromStats.meanProcessBodyTime)
	prometheus.MustRegister(globalPromStats.warcWritingQueueSize)
	prometheus.MustRegister(globalPromStats.meanWaitOnFeedbackTime)

	// Register dedup WARC metrics
	prometheus.MustRegister(globalPromStats.dataTotalBytes)
	prometheus.MustRegister(globalPromStats.dataTotalBytesContentLength)
	prometheus.MustRegister(globalPromStats.cdxDedupeTotalBytes)
	prometheus.MustRegister(globalPromStats.doppelgangerDedupeTotalBytes)
	prometheus.MustRegister(globalPromStats.localDedupeTotalBytes)
	prometheus.MustRegister(globalPromStats.cdxDedupeTotal)
	prometheus.MustRegister(globalPromStats.doppelgangerDedupeTotal)
	prometheus.MustRegister(globalPromStats.localDedupeTotal)
}

func PrometheusHandler() http.Handler {
	return promhttp.Handler()
}
