// Package crawl handles all the crawling logic for Zeno
package crawl

import (
	"path"
	"sync"
	"time"

	"git.archive.org/wb/gocrawlhq"
	"github.com/CorentinB/warc"
	"github.com/internetarchive/Zeno/internal/pkg/queue"
	"github.com/internetarchive/Zeno/internal/pkg/seencheck"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
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

	// Initialize the queue & seencheck
	c.Log.Info("Initializing queue and seencheck..")
	c.Queue, err = queue.NewPersistentGroupedQueue(path.Join(c.JobPath, "queue"), c.UseHandover, c.UseCommit)
	if err != nil {
		c.Log.Fatal("unable to init queue", "error", err)
	}

	c.Seencheck, err = seencheck.New(c.JobPath)
	if err != nil {
		c.Log.Fatal("unable to init seencheck", "error", err)
	}

	// Start the background process that will periodically check if the disk
	// have enough free space, and potentially pause the crawl if it doesn't
	go c.handleCrawlPause()

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

	// Start the process responsible for printing live stats on the standard output
	if c.LiveStats {
		go c.printLiveStats()
	}

	// If crawl HQ parameters are specified, then we start the background
	// processes responsible for pulling and pushing seeds from and to HQ
	if c.UseHQ {
		c.HQClient, err = gocrawlhq.Init(c.HQKey, c.HQSecret, c.HQProject, c.HQAddress)
		if err != nil {
			c.Log.Fatal("unable to init crawl HQ client", "error", err)
		}

		c.HQProducerChannel = make(chan *queue.Item, c.Workers.Count)
		c.HQFinishedChannel = make(chan *queue.Item, c.Workers.Count)

		c.HQChannelsWg.Add(2)
		go c.HQConsumer()
		go c.HQProducer()
		go c.HQFinisher()
		go c.HQWebsocket()
	} else {
		// Temporarily disable handover as it's not needed
		enableBackHandover := make(chan struct{})
		syncHandover := make(chan struct{})
		if c.UseHandover {
			c.Log.Info("Temporarily disabling handover..")

			go c.Queue.TempDisableHandover(enableBackHandover, syncHandover)

			<-syncHandover
		}
		// Push the seed list to the queue
		c.Log.Info("Pushing seeds in the local queue..")
		for i := 0; i < len(c.SeedList); i += 100000 {
			end := i + 100000
			if end > len(c.SeedList) {
				end = len(c.SeedList)
			}

			c.Log.Info("Enqueuing seeds", "index", i)

			// Create a slice of pointers to the items in the current batch
			seedPointers := make([]*queue.Item, end-i)
			for j := range seedPointers {
				seedPointers[j] = &c.SeedList[i+j]
			}

			if err := c.Queue.BatchEnqueue(seedPointers...); err != nil {
				c.Log.Error("unable to enqueue seeds, discarding", "error", err)
			}
		}

		c.SeedList = nil

		// Re-enable handover
		if c.UseHandover {
			c.Log.Info("Enabling handover..")
			enableBackHandover <- struct{}{}
			<-syncHandover
		}
		close(enableBackHandover)
		close(syncHandover)

		c.Log.Info("All seeds are now in queue")
	}

	// Start the workers pool by building all the workers and starting them
	// Also starts all the background processes that will handle the workers
	c.Workers.Start()

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
