package stats

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type promStats struct {
	urlCrawled            prometheus.Counter
	finishedSeeds         prometheus.Counter
	preprocessorRoutines  prometheus.Gauge
	archiverRoutines      prometheus.Gauge
	postprocessorRoutines prometheus.Gauge
	finisherRoutines      prometheus.Gauge
	paused                prometheus.Gauge
	http2xx               prometheus.Counter
	http3xx               prometheus.Counter
	http4xx               prometheus.Counter
	http5xx               prometheus.Counter
	meanHTTPRespTime      prometheus.Gauge
	warcWritingQueueSize  prometheus.Gauge
}

func registerPromMetrics() {
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

func PromHandler() http.Handler {
	return promhttp.Handler()
}
