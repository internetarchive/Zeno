package stats

import (
	"net/http"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type prometheusStats struct {
	urlCrawled            *prometheus.CounterVec
	finishedSeeds         *prometheus.CounterVec
	preprocessorRoutines  *prometheus.GaugeVec
	archiverRoutines      *prometheus.GaugeVec
	postprocessorRoutines *prometheus.GaugeVec
	finisherRoutines      *prometheus.GaugeVec
	paused                *prometheus.GaugeVec
	http2xx               *prometheus.CounterVec
	http3xx               *prometheus.CounterVec
	http4xx               *prometheus.CounterVec
	http5xx               *prometheus.CounterVec
	meanHTTPRespTime      *prometheus.GaugeVec
	warcWritingQueueSize  *prometheus.GaugeVec
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
		meanHTTPRespTime: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "mean_http_resp_time", Help: "Mean HTTP response time"},
			[]string{"project", "hostname", "version"},
		),
		warcWritingQueueSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: config.Get().PrometheusPrefix + "warc_writing_queue_size", Help: "Size of the WARC writing queue"},
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
	prometheus.MustRegister(globalPromStats.warcWritingQueueSize)
}

func PrometheusHandler() http.Handler {
	return promhttp.Handler()
}
