package crawl

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

<<<<<<< HEAD
<<<<<<< HEAD
	"net/http"
	_ "net/http/pprof"

=======
	"github.com/CorentinB/warc"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
>>>>>>> e5c3f71 (Bump warc lib to 0.8.40 (#76))
=======
	"net/http"
	_ "net/http/pprof"

>>>>>>> 1dd291d (Remove Gin dependency)
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// APIWorkersState represents the state of all API workers.
type APIWorkersState struct {
	Workers []*APIWorkerState `json:"workers"`
}

// APIWorkerState represents the state of an API worker.
type APIWorkerState struct {
	WorkerID  uint   `json:"worker_id"`
	Status    string `json:"status"`
	LastError string `json:"last_error"`
	LastSeen  string `json:"last_seen"`
	Locked    bool   `json:"locked"`
}

// startAPI starts the API server for the crawl.
func (crawl *Crawl) startAPI() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		crawledSeeds := crawl.CrawledSeeds.Value()
		crawledAssets := crawl.CrawledAssets.Value()

<<<<<<< HEAD
<<<<<<< HEAD
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := map[string]interface{}{
			"rate":          crawl.URIsPerSecond.Rate(),
			"crawled":       crawledSeeds + crawledAssets,
			"crawledSeeds":  crawledSeeds,
			"crawledAssets": crawledAssets,
			"queued":        crawl.Frontier.QueueCount.Value(),
			"uptime":        time.Since(crawl.StartTime).String(),
=======
=======
>>>>>>> 1dd291d (Remove Gin dependency)
		c.JSON(200, gin.H{
			"rate":                crawl.URIsPerSecond.Rate(),
			"crawled":             crawledSeeds + crawledAssets,
			"crawled_seeds":       crawledSeeds,
			"crawled_assets":      crawledAssets,
			"queued":              crawl.Frontier.QueueCount.Value(),
			"data_written":        warc.DataTotal.Value(),
			"data_deduped_remote": warc.RemoteDedupeTotal.Value(),
			"data_deduped_local":  warc.LocalDedupeTotal.Value(),
			"uptime":              time.Since(crawl.StartTime).String(),
		})
	})

	// Handle Prometheus export
	if crawl.Prometheus {
		labels := make(map[string]string)

		labels["crawljob"] = crawl.Job
		hostname, err := os.Hostname()
		if err != nil {
			crawl.Log.Warn("Unable to retrieve hostname of machine")
			hostname = "unknown"
<<<<<<< HEAD
>>>>>>> e5c3f71 (Bump warc lib to 0.8.40 (#76))
=======
=======
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := map[string]interface{}{
			"rate":          crawl.URIsPerSecond.Rate(),
			"crawled":       crawledSeeds + crawledAssets,
			"crawledSeeds":  crawledSeeds,
			"crawledAssets": crawledAssets,
			"queued":        crawl.Frontier.QueueCount.Value(),
			"uptime":        time.Since(crawl.StartTime).String(),
>>>>>>> 890c394 (Remove Gin dependency)
>>>>>>> 1dd291d (Remove Gin dependency)
		}

		json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/metrics", setupPrometheus(crawl).ServeHTTP)

	http.HandleFunc("/workers", func(w http.ResponseWriter, r *http.Request) {
		workersState := crawl.GetWorkerState(-1)
		json.NewEncoder(w).Encode(workersState)
	})

	http.HandleFunc("/worker/", func(w http.ResponseWriter, r *http.Request) {
		workerID := strings.TrimPrefix(r.URL.Path, "/worker/")
		workerIDInt, err := strconv.Atoi(workerID)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Unsupported worker ID",
			})
			return
		}

		workersState := crawl.GetWorkerState(workerIDInt)
		if workersState == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Worker not found",
			})
			return
		}

		json.NewEncoder(w).Encode(workersState)
	})

	err := http.ListenAndServe(":"+crawl.APIPort, nil)
	if err != nil {
		crawl.Log.Fatal("unable to start API", "error", err.Error())
	}
}

func setupPrometheus(crawl *Crawl) http.Handler {
	labels := make(map[string]string)

	labels["crawljob"] = crawl.Job
	hostname, err := os.Hostname()
	if err != nil {
		crawl.Log.Warn("Unable to retrieve hostname of machine")
		hostname = "unknown"
	}
	labels["host"] = hostname + ":" + crawl.APIPort

	crawl.PrometheusMetrics.DownloadedURI = promauto.NewCounter(prometheus.CounterOpts{
		Name:        crawl.PrometheusMetrics.Prefix + "downloaded_uri_count_total",
		ConstLabels: labels,
		Help:        "The total number of crawled URI",
	})

	crawl.Log.Info("starting Prometheus export")

	return promhttp.Handler()
}
