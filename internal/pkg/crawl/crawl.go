// Package crawl handles all the crawling logic for Zeno
package crawl

import (
	"fmt"
	"sync"
	"time"

	"git.archive.org/wb/gocrawlhq"
	"github.com/CorentinB/warc"
	"github.com/internetarchive/Zeno/internal/pkg/frontier"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/telanflow/cookiejar"
	"mvdan.cc/xurls/v2"
)

// PrometheusMetrics define all the metrics exposed by the Prometheus exporter
type PrometheusMetrics struct {
	Prefix        string
	DownloadedURI prometheus.Counter
}

// Start fire up the crawling process
func (c *Crawl) Start() (err error) {
	c.StartTime = time.Now()
	c.Paused = new(utils.TAtomBool)
	c.Finished = new(utils.TAtomBool)
	c.HQChannelsWg = new(sync.WaitGroup)
	regexOutlinks = xurls.Relaxed()

	// Setup the --crawl-time-limit clock
	if c.CrawlTimeLimit != 0 {
		go func() {
			time.Sleep(time.Second * time.Duration(c.CrawlTimeLimit))
			c.Log.Info("Crawl time limit reached: attempting to finish the crawl.")
			go c.finish()
			time.Sleep((time.Duration(c.MaxCrawlTimeLimit) * time.Second) - (time.Duration(c.CrawlTimeLimit) * time.Second))
			c.Log.Fatal("Max crawl time limit reached, exiting..")
		}()
	}

	// Start the background process that will handle os signals
	// to exit Zeno, like CTRL+C
	go c.setupCloseHandler()

	// Initialize the frontier
	frontierLoggingChan := make(chan *frontier.FrontierLogMessage, 10)
	go func() {
		for log := range frontierLoggingChan {
			switch log.Level {
			case logrus.ErrorLevel:
				c.Log.WithFields(c.genLogFields(nil, nil, log.Fields)).Error(log.Message)
			case logrus.WarnLevel:
				c.Log.WithFields(c.genLogFields(nil, nil, log.Fields)).Warn(log.Message)
			case logrus.InfoLevel:
				c.Log.WithFields(c.genLogFields(nil, nil, log.Fields)).Info(log.Message)
			}
		}
	}()

	c.Frontier.Init(c.JobPath, frontierLoggingChan, c.Workers, c.Seencheck)
	c.Frontier.Load()
	c.Frontier.Start()

	// Start the background process that will periodically check if the disk
	// have enough free space, and potentially pause the crawl if it doesn't
	go c.handleCrawlPause()

	// Function responsible for writing to disk the frontier's hosts pool
	// and other stats needed to resume the crawl. The process happen every minute.
	// The actual queue used during the crawl and seencheck aren't included in this,
	// because they are written to disk in real-time.
	go c.writeFrontierToDisk()

	// Initialize WARC writer
	c.Log.Info("Initializing WARC writer..")

	// Init WARC rotator settings
	rotatorSettings := c.initWARCRotatorSettings()

	dedupeOptions := warc.DedupeOptions{LocalDedupe: !c.DisableLocalDedupe, SizeThreshold: c.WARCDedupSize}
	if c.CDXDedupeServer != "" {
		dedupeOptions = warc.DedupeOptions{LocalDedupe: !c.DisableLocalDedupe, CDXDedupe: true, CDXURL: c.CDXDedupeServer, CDXCookie: c.WARCCustomCookie, SizeThreshold: c.WARCDedupSize}
	}

	// Init the HTTP client responsible for recording HTTP(s) requests / responses
	HTTPClientSettings := warc.HTTPClientSettings{
		RotatorSettings:     rotatorSettings,
		DedupeOptions:       dedupeOptions,
		DecompressBody:      true,
		SkipHTTPStatusCodes: []int{429},
		VerifyCerts:         c.CertValidation,
		TempDir:             c.WARCTempDir,
		FullOnDisk:          c.WARCFullOnDisk,
		RandomLocalIP:       c.RandomLocalIP,
	}

	c.Client, err = warc.NewWARCWritingHTTPClient(HTTPClientSettings)
	if err != nil {
		c.Log.Fatal("Unable to init WARC writing HTTP client", "error", err)
	}

	go func() {
		for err := range c.Client.ErrChan {
			c.Log.WithFields(c.genLogFields(err, nil, nil)).Error("WARC HTTP client error")
		}
	}()

	c.Client.Timeout = time.Duration(c.HTTPTimeout) * time.Second
	c.Log.Info("HTTP client timeout set", "timeout", c.HTTPTimeout)

	if c.Proxy != "" {
		proxyHTTPClientSettings := HTTPClientSettings
		proxyHTTPClientSettings.Proxy = c.Proxy

		c.ClientProxied, err = warc.NewWARCWritingHTTPClient(proxyHTTPClientSettings)
		if err != nil {
			c.Log.Fatal("unable to init WARC writing (proxy) HTTP client")
		}

		go func() {
			for err := range c.ClientProxied.ErrChan {
				c.Log.WithFields(c.genLogFields(err, nil, nil)).Error("WARC HTTP client error")
			}
		}()
	}

	c.Log.Info("WARC writer initialized")

	// Process responsible for slowing or pausing the crawl
	// when the WARC writing queue gets too big
	go c.crawlSpeedLimiter()

	if c.API {
		go c.startAPI()
	}

	// Parse input cookie file if specified
	if c.CookieFile != "" {
		cookieJar, err := cookiejar.NewFileJar(c.CookieFile, nil)
		if err != nil {
			c.Log.WithFields(c.genLogFields(err, nil, nil)).Fatal("unable to parse cookie file")
		}

		c.Client.Jar = cookieJar
	}

	// Fire up the desired amount of workers
	for i := uint(0); i < uint(c.Workers); i++ {
		worker := newWorker(c, i)
		c.WorkerPool = append(c.WorkerPool, worker)
		go worker.Run()
	}
	go c.WorkerWatcher()

	// Start the process responsible for printing live stats on the standard output
	if c.LiveStats {
		go c.printLiveStats()
	}

	// If crawl HQ parameters are specified, then we start the background
	// processes responsible for pulling and pushing seeds from and to HQ
	if c.UseHQ {
		c.HQClient, err = gocrawlhq.Init(c.HQKey, c.HQSecret, c.HQProject, c.HQAddress)
		if err != nil {
			logrus.Panic(err)
		}

		c.HQProducerChannel = make(chan *frontier.Item, c.Workers)
		c.HQFinishedChannel = make(chan *frontier.Item, c.Workers)

		c.HQChannelsWg.Add(2)
		go c.HQConsumer()
		go c.HQProducer()
		go c.HQFinisher()
		go c.HQWebsocket()
	} else {
		// Push the seed list to the queue
		c.Log.Info("Pushing seeds in the local queue..")
		for _, item := range c.SeedList {
			item := item
			c.Frontier.PushChan <- &item
		}
		c.SeedList = nil
		c.Log.Info("All seeds are now in queue, crawling will start")
	}

	// Start the background process that will catch when there
	// is nothing more to crawl
	if !c.UseHQ {
		c.catchFinish()
	} else {
		for {
			time.Sleep(time.Second)
		}
	}

	return
}

// WorkerWatcher is a background process that watches over the workers
// and remove them from the pool when they are done
func (c *Crawl) WorkerWatcher() {
	var toEnd = false

	for {
		select {

		// Stop the workers when requested
		case <-c.WorkerStopSignal:
			for _, worker := range c.WorkerPool {
				worker.doneSignal <- true
			}
			toEnd = true

		// Check for finished workers and remove them from the pool
		// End the watcher if a stop signal was received beforehand and all workers are completed
		default:
			c.WorkerMutex.Lock()
			for i, worker := range c.WorkerPool {
				if worker.state.status == completed {
					// Remove the worker from the pool
					c.WorkerPool = append(c.WorkerPool[:i], c.WorkerPool[i+1:]...)
				}
				worker.id = uint(i)
			}

			if toEnd && len(c.WorkerPool) == 0 {
				c.WorkerMutex.Unlock()
				return // All workers are completed
			}
			c.WorkerMutex.Unlock()
		}
	}
}

// EnsureWorkersFinished waits for all workers to finish
func (c *Crawl) EnsureWorkersFinished() bool {
	var workerPoolLen int
	var timer = time.NewTimer(c.WorkerStopTimeout)

	for {
		c.WorkerMutex.RLock()
		workerPoolLen = len(c.WorkerPool)
		if workerPoolLen == 0 {
			c.WorkerMutex.RUnlock()
			return true
		}
		c.WorkerMutex.RUnlock()
		select {
		case <-timer.C:
			c.Log.Warn(fmt.Sprintf("[WORKERS] Timeout reached. %d workers still running", workerPoolLen))
			return false
		default:
			c.Log.Warn(fmt.Sprintf("[WORKERS] Waiting for %d workers to finish", workerPoolLen))
			time.Sleep(time.Second * 5)
		}
	}
}

// GetWorkerState returns the state of a worker given its index in the worker pool
// if the provided index is -1 then the state of all workers is returned
func (c *Crawl) GetWorkerState(index int) interface{} {
	c.WorkerMutex.RLock()
	defer c.WorkerMutex.RUnlock()

	if index == -1 {
		var workersStatus = new(APIWorkersState)
		for _, worker := range c.WorkerPool {
			workersStatus.Workers = append(workersStatus.Workers, _getWorkerState(worker))
		}
		return workersStatus
	}
	if index >= len(c.WorkerPool) {
		return nil
	}
	return _getWorkerState(c.WorkerPool[index])
}

func _getWorkerState(worker *Worker) *APIWorkerState {
	lastErr := ""
	isLocked := true

	if worker.TryLock() {
		isLocked = false
		worker.Unlock()
	}

	if worker.state.lastError != nil {
		lastErr = worker.state.lastError.Error()
	}

	return &APIWorkerState{
		WorkerID:  worker.id,
		Status:    worker.state.status.String(),
		LastSeen:  worker.state.lastSeen.Format(time.RFC3339),
		LastError: lastErr,
		Locked:    isLocked,
	}
}
