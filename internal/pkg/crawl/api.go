package crawl

import (
	"fmt"
	"time"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (crawl *Crawl) startAPI() {
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = logInfo.Out

	r := gin.Default()

	pprof.Register(r)

	logInfo.Info("Starting API")
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"rate":         crawl.URIsPerSecond.Rate(),
			"crawled":      crawl.Crawled.Value(),
			"queued":       crawl.Frontier.QueueCount.Value(),
			"running_time": fmt.Sprintf("%s", time.Since(crawl.StartTime)),
		})
	})

	// Handle Prometheus export
	if crawl.Prometheus {
		logInfo.Info("Starting Prometheus export")

		crawl.PrometheusMetrics.DownloadedURI = promauto.NewCounter(prometheus.CounterOpts{
			Name: crawl.PrometheusMetrics.JobName + "_downloaded_uri_count_total",
			Help: "The total number of crawled URI",
		})

		r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	r.Run(":9443")
}
