package crawl

import (
	"net/http"
	"sync"
	"time"

	"git.archive.org/wb/gocrawlhq"
	"github.com/CorentinB/warc"
	"github.com/internetarchive/Zeno/config/v2"
	"github.com/internetarchive/Zeno/internal/pkg/frontier"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/paulbellamy/ratecounter"
)

// Crawl define the parameters of a crawl process
type Crawl struct {
	*sync.Mutex
	StartTime time.Time
	SeedList  []frontier.Item
	Paused    *utils.TAtomBool
	Finished  *utils.TAtomBool
	LiveStats bool

	// Logger
	Log *log.Logger

	// Frontier
	Frontier *frontier.Frontier

	// Worker pool
	WorkerMutex       sync.RWMutex
	WorkerPool        []*Worker
	WorkerStopSignal  chan bool
	WorkerStopTimeout time.Duration

	// Crawl settings
	MaxConcurrentAssets            int
	Client                         *warc.CustomHTTPClient
	ClientProxied                  *warc.CustomHTTPClient
	DisabledHTMLTags               []string
	ExcludedHosts                  []string
	IncludedHosts                  []string
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
	CrawlTimeLimit                 int
	MaxCrawlTimeLimit              int
	DisableAssetsCapture           bool
	CaptureAlternatePages          bool
	DomainsCrawl                   bool
	Headless                       bool
	Seencheck                      bool
	Workers                        int
	RandomLocalIP                  bool

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
	WARCCustomCookie   string

	// Crawl HQ settings
	UseHQ                  bool
	HQAddress              string
	HQProject              string
	HQKey                  string
	HQSecret               string
	HQStrategy             string
	HQBatchSize            int
	HQContinuousPull       bool
	HQClient               *gocrawlhq.Client
	HQFinishedChannel      chan *frontier.Item
	HQProducerChannel      chan *frontier.Item
	HQChannelsWg           *sync.WaitGroup
	HQRateLimitingSendBack bool
}

func GenerateCrawlConfig(config *config.Config) (*Crawl, error) {
	return nil, nil
}
