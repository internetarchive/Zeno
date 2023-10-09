package crawl

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"git.archive.org/wb/gocrawlhq"
	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/CorentinB/warc"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	cookiejar "github.com/orirawlings/persistent-cookiejar"
	"github.com/paulbellamy/ratecounter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/remeh/sizedwaitgroup"
	"github.com/sirupsen/logrus"
	"mvdan.cc/xurls/v2"
)

var (
	logInfo    *logrus.Logger
	logWarning *logrus.Logger
	logError   *logrus.Logger
)

// PrometheusMetrics define all the metrics exposed by the Prometheus exporter
type PrometheusMetrics struct {
	Prefix        string
	DownloadedURI prometheus.Counter
}

// Crawl define the parameters of a crawl process
type Crawl struct {
	*sync.Mutex
	StartTime        time.Time
	SeedList         []frontier.Item
	Paused           *utils.TAtomBool
	Finished         *utils.TAtomBool
	LiveStats        bool
	ElasticSearchURL string

	// Frontier
	Frontier *frontier.Frontier

	// Crawl settings
	WorkerPool                     sizedwaitgroup.SizedWaitGroup
	MaxConcurrentAssets            int
	Client                         *warc.CustomHTTPClient
	ClientProxied                  *warc.CustomHTTPClient
	Logger                         logrus.Logger
	DisabledHTMLTags               []string
	ExcludedHosts                  []string
	ExcludedStrings                []string
	UserAgent                      string
	Job                            string
	JobPath                        string
	MaxHops                        uint8
	MaxRetry                       int
	MaxRedirect                    int
	HTTPTimeout                    int
	MaxConcurrentRequestsPerDomain int
	RateLimitDelay                 int
	DisableAssetsCapture           bool
	CaptureAlternatePages          bool
	DomainsCrawl                   bool
	Seencheck                      bool
	Workers                        int

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

	// Headless browser stuff
	HeadlessBrowser       *rod.Browser
	Headless              bool
	Headfull              bool
	HeadlessWaitAfterLoad uint64
}

// Start fire up the crawling process
func (c *Crawl) Start() (err error) {
	c.StartTime = time.Now()
	c.Paused = new(utils.TAtomBool)
	c.Finished = new(utils.TAtomBool)
	c.HQChannelsWg = new(sync.WaitGroup)
	regexOutlinks = xurls.Relaxed()

	// Setup logging, every day at midnight UTC a new setup
	// is triggered in order to change the ES index's name
	if c.ElasticSearchURL != "" {
		// Goroutine loop that fetch the machine's IP address every second
		go func() {
			for {
				ip := utils.GetOutboundIP().String()
				constants.Store("ip", ip)
				time.Sleep(time.Second * 10)
			}
		}()

		logInfo, logWarning, logError = utils.SetupLogging(c.JobPath, c.LiveStats, c.ElasticSearchURL)

		go func() {
			// Get the current time in UTC and figure out when the next midnight will occur
			now := time.Now().UTC()
			midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			if now.After(midnight) {
				midnight = midnight.Add(24 * time.Hour)
			}

			// Calculate the duration until midnight and add a little extra time to avoid calling your function just before midnight
			duration := midnight.Sub(now) + time.Second*10

			// Create a timer that will wait until midnight
			timer := time.NewTimer(duration)

			// Wait for the timer to finish (which will occur at midnight)
			<-timer.C

			// Call your function
			logInfo, logWarning, logError = utils.SetupLogging(c.JobPath, c.LiveStats, c.ElasticSearchURL)
		}()
	} else {
		logInfo, logWarning, logError = utils.SetupLogging(c.JobPath, c.LiveStats, c.ElasticSearchURL)
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
				logError.WithFields(c.genLogFields(nil, nil, log.Fields)).Error(log.Message)
			case logrus.WarnLevel:
				logWarning.WithFields(c.genLogFields(nil, nil, log.Fields)).Warn(log.Message)
			case logrus.InfoLevel:
				logInfo.WithFields(c.genLogFields(nil, nil, log.Fields)).Info(log.Message)
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
			logError.WithFields(c.genLogFields(err, nil, nil)).Errorf("WARC HTTP client error")
		}
	}()

	c.Client.Timeout = time.Duration(c.HTTPTimeout) * time.Second
	logrus.Infof("HTTP client timeout set to %d seconds", c.HTTPTimeout)

	if c.Proxy != "" {
		proxyHTTPClientSettings := HTTPClientSettings
		proxyHTTPClientSettings.Proxy = c.Proxy

		c.ClientProxied, err = warc.NewWARCWritingHTTPClient(proxyHTTPClientSettings)
		if err != nil {
			logError.Fatal("unable to init WARC writing (proxy) HTTP client")
		}

		go func() {
			for err := range c.ClientProxied.ErrChan {
				logError.WithFields(c.genLogFields(err, nil, nil)).Error("WARC HTTP client error")
			}
		}()
	}

	logrus.Info("WARC writer initialized")

	// Process responsible for slowing or pausing the crawl
	// when the WARC writing queue gets too big
	go c.crawlSpeedLimiter()

	// Starting the headless browser if needed
	if c.Headless {
		if c.Headfull {
			logrus.Info("Starting headless browser in headfull mode")
		} else {
			logrus.Info("Starting headless browser in headless mode")
		}

		l := launcher.New().
			// Set(flags.Headless, "new").
			Headless(!c.Headfull).
			Devtools(false)
		defer l.Cleanup()

		controlURL := l.MustLaunch()

		c.HeadlessBrowser = rod.New().
			ControlURL(controlURL).
			MustConnect()
	}

	if c.API {
		go c.startAPI()
	}

	// Parse input cookie file if specified
	if c.CookieFile != "" {
		jar, err := cookiejar.New(&cookiejar.Options{
			PublicSuffixList:      nil,
			Filename:              c.CookieFile,
			PersistSessionCookies: true,
		})
		if err != nil {
			logError.WithFields(c.genLogFields(err, nil, nil)).Error("unable to create cookie jar")
			return err
		}

		fmt.Println(jar.AllCookies())

		// Fake FB URL
		u, _ := url.Parse("https://www.facebook.com/")
		if err != nil {
			logError.WithFields(c.genLogFields(err, nil, nil)).Fatal("unable to parse cookie file")
		}

		logrus.Panic("Cookie jar: ", jar.Cookies(u))

		c.Client.Jar = jar
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
		go c.HQWebsocket()
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
