package crawl

import (
	"net/http"
	"sync"
	"time"

	"git.archive.org/wb/gocrawlhq"
	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/CorentinB/warc"
	"github.com/paulbellamy/ratecounter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/remeh/sizedwaitgroup"
	"github.com/sirupsen/logrus"
	"github.com/telanflow/cookiejar"
	"mvdan.cc/xurls/v2"
)

var logInfo *logrus.Logger
var logWarning *logrus.Logger

// PrometheusMetrics define all the metrics exposed by the Prometheus exporter
type PrometheusMetrics struct {
	Prefix        string
	DownloadedURI prometheus.Counter
}

// Crawl define the parameters of a crawl process
type Crawl struct {
	*sync.Mutex
	StartTime time.Time
	SeedList  []frontier.Item
	Paused    *utils.TAtomBool
	Finished  *utils.TAtomBool
	LiveStats bool

	// Frontier
	Frontier *frontier.Frontier

	// Crawl settings
	WorkerPool            sizedwaitgroup.SizedWaitGroup
	MaxConcurrentAssets   int
	Client                *warc.CustomHTTPClient
	ClientProxied         *warc.CustomHTTPClient
	Logger                logrus.Logger
	DisabledHTMLTags      []string
	ExcludedHosts         []string
	UserAgent             string
	Job                   string
	JobPath               string
	MaxHops               uint8
	MaxRetry              int
	MaxRedirect           int
	HTTPTimeout           int
	DisableAssetsCapture  bool
	CaptureAlternatePages bool
	DomainsCrawl          bool
	Headless              bool
	Seencheck             bool
	Workers               int

	// Cookie-related settings
	CookieFile  string
	KeepCookies bool
	CookieJar   http.CookieJar

	// proxy settings
	Proxy       string
	BypassProxy []string

	// API settings
	API               bool
	APIPort           string
	Prometheus        bool
	PrometheusMetrics *PrometheusMetrics

	// Real time statistics
	URIsPerSecond *ratecounter.RateCounter
	ActiveWorkers *ratecounter.Counter
	CrawledSeeds  *ratecounter.Counter
	CrawledAssets *ratecounter.Counter

	// WARC settings
	WARCPrefix         string
	WARCOperator       string
	WARCWriter         chan *warc.RecordBatch
	WARCWriterFinish   chan bool
	WARCTempDir        string
	CDXDedupeServer    string
	WARCFullOnDisk     bool
	WARCPoolSize       int
	WARCDedupSize      int
	DisableLocalDedupe bool
	CertValidation     bool

	// Crawl HQ settings
	UseHQ             bool
	HQAddress         string
	HQProject         string
	HQKey             string
	HQSecret          string
	HQStrategy        string
	HQBatchSize       int
	HQContinuousPull  bool
	HQClient          *gocrawlhq.Client
	HQFinishedChannel chan *frontier.Item
	HQProducerChannel chan *frontier.Item
	HQChannelsWg      *sync.WaitGroup
}

// Start fire up the crawling process
func (c *Crawl) Start() (err error) {
	c.StartTime = time.Now()
	c.Paused = new(utils.TAtomBool)
	c.Finished = new(utils.TAtomBool)
	c.HQChannelsWg = new(sync.WaitGroup)
	regexOutlinks = xurls.Relaxed()

	// Setup logging
	logInfo, logWarning = utils.SetupLogging(c.JobPath, c.LiveStats)

	// Start the background process that will handle os signals
	// to exit Zeno, like CTRL+C
	go c.setupCloseHandler()

	// Initialize the frontier
	c.Frontier.Init(c.JobPath, logInfo, logWarning, c.Workers, c.Seencheck)
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
	logrus.Info("Initializing WARC writer..")

	// Init WARC rotator settings
	rotatorSettings := c.initWARCRotatorSettings()

	// Change WARC pool size
	rotatorSettings.WARCWriterPoolSize = c.WARCPoolSize

	dedupeOptions := warc.DedupeOptions{LocalDedupe: !c.DisableLocalDedupe, SizeThreshold: c.WARCDedupSize}
	if c.CDXDedupeServer != "" {
		dedupeOptions = warc.DedupeOptions{LocalDedupe: !c.DisableLocalDedupe, CDXDedupe: true, CDXURL: c.CDXDedupeServer, SizeThreshold: c.WARCDedupSize}
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
	}

	c.Client, err = warc.NewWARCWritingHTTPClient(HTTPClientSettings)
	if err != nil {
		logrus.Fatalf("Unable to init WARC writing HTTP client: %s", err)
	}

	go func() {
		for err := range c.Client.ErrChan {
			logWarning.Errorf("WARC HTTP client error: %s", err.Err.Error())
		}
	}()

	c.Client.Timeout = time.Duration(c.HTTPTimeout) * time.Second
	logrus.Infof("HTTP client timeout set to %d seconds", c.HTTPTimeout)

	if c.Proxy != "" {
		proxyHTTPClientSettings := HTTPClientSettings
		proxyHTTPClientSettings.Proxy = c.Proxy

		c.ClientProxied, err = warc.NewWARCWritingHTTPClient(proxyHTTPClientSettings)
		if err != nil {
			logrus.Fatalf("Unable to init WARC writing (proxy) HTTP client: %s", err)
		}

		go func() {
			for err := range c.ClientProxied.ErrChan {
				logWarning.Errorf("WARC HTTP client error: %s", err.Err.Error())
			}
		}()
	}

	logrus.Info("WARC writer initialized")

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
			panic(err)
		}

		c.Client.Jar = cookieJar
	}

	// Fire up the desired amount of workers
	for i := 0; i < c.Workers; i++ {
		c.WorkerPool.Add()
		go c.Worker()
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
			logrus.Panic(err)
		}

		c.HQProducerChannel = make(chan *frontier.Item, c.Workers)
		c.HQFinishedChannel = make(chan *frontier.Item, c.Workers)

		c.HQChannelsWg.Add(2)
		go c.HQConsumer()
		go c.HQProducer()
		go c.HQFinisher()
	} else {
		// Push the seed list to the queue
		logrus.Info("Pushing seeds in the local queue..")
		for _, item := range c.SeedList {
			item := item
			c.Frontier.PushChan <- &item
		}
		c.SeedList = nil
		logrus.Info("All seeds are now in queue, crawling will start")
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
