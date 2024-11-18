package config

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/CorentinB/warc"
	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/gocrawlhq"
	"github.com/paulbellamy/ratecounter"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config holds all configuration for our program, parsed from various sources
// The `mapstructure` tags are used to map the fields to the viper configuration
type Config struct {
	LogLevel                       string   `mapstructure:"log-level"`
	UserAgent                      string   `mapstructure:"user-agent"`
	Job                            string   `mapstructure:"job"`
	Cookies                        string   `mapstructure:"cookies"`
	APIPort                        string   `mapstructure:"api-port"`
	PrometheusPrefix               string   `mapstructure:"prometheus-prefix"`
	WARCPrefix                     string   `mapstructure:"warc-prefix"`
	WARCOperator                   string   `mapstructure:"warc-operator"`
	CDXDedupeServer                string   `mapstructure:"warc-cdx-dedupe-server"`
	WARCTempDir                    string   `mapstructure:"warc-temp-dir"`
	WARCSize                       int      `mapstructure:"warc-size"`
	CDXCookie                      string   `mapstructure:"cdx-cookie"`
	HQAddress                      string   `mapstructure:"hq-address"`
	HQKey                          string   `mapstructure:"hq-key"`
	HQSecret                       string   `mapstructure:"hq-secret"`
	HQProject                      string   `mapstructure:"hq-project"`
	HQStrategy                     string   `mapstructure:"hq-strategy"`
	HQBatchSize                    int64    `mapstructure:"hq-batch-size"`
	HQBatchConcurrency             int      `mapstructure:"hq-batch-concurrency"`
	LogFileOutputDir               string   `mapstructure:"log-file-output-dir"`
	ElasticSearchUsername          string   `mapstructure:"es-user"`
	ElasticSearchPassword          string   `mapstructure:"es-password"`
	ElasticSearchIndexPrefix       string   `mapstructure:"es-index-prefix"`
	DisableHTMLTag                 []string `mapstructure:"disable-html-tag"`
	ExcludeHosts                   []string `mapstructure:"exclude-host"`
	IncludeHosts                   []string `mapstructure:"include-host"`
	IncludeString                  []string `mapstructure:"include-string"`
	ExcludeString                  []string `mapstructure:"exclude-string"`
	ElasticSearchURLs              []string `mapstructure:"es-url"`
	WorkersCount                   int      `mapstructure:"workers"`
	MaxConcurrentAssets            int      `mapstructure:"max-concurrent-assets"`
	MaxHops                        uint8    `mapstructure:"max-hops"`
	MaxRedirect                    uint8    `mapstructure:"max-redirect"`
	MaxRetry                       uint8    `mapstructure:"max-retry"`
	HTTPTimeout                    int      `mapstructure:"http-timeout"`
	MaxConcurrentRequestsPerDomain int      `mapstructure:"max-concurrent-per-domain"`
	ConcurrentSleepLength          int      `mapstructure:"concurrent-sleep-length"`
	CrawlTimeLimit                 int      `mapstructure:"crawl-time-limit"`
	CrawlMaxTimeLimit              int      `mapstructure:"crawl-max-time-limit"`
	MinSpaceRequired               int      `mapstructure:"min-space-required"`
	WARCPoolSize                   int      `mapstructure:"warc-pool-size"`
	WARCDedupeSize                 int      `mapstructure:"warc-dedupe-size"`
	KeepCookies                    bool     `mapstructure:"keep-cookies"`
	Headless                       bool     `mapstructure:"headless"`
	DisableSeencheck               bool     `mapstructure:"disable-seencheck"`
	JSON                           bool     `mapstructure:"json"`
	Debug                          bool     `mapstructure:"debug"`
	LiveStats                      bool     `mapstructure:"live-stats"`
	API                            bool     `mapstructure:"api"`
	Prometheus                     bool     `mapstructure:"prometheus"`
	DomainsCrawl                   bool     `mapstructure:"domains-crawl"`
	CaptureAlternatePages          bool     `mapstructure:"capture-alternate-pages"`
	WARCOnDisk                     bool     `mapstructure:"warc-on-disk"`
	DisableLocalDedupe             bool     `mapstructure:"disable-local-dedupe"`
	CertValidation                 bool     `mapstructure:"cert-validation"`
	DisableAssetsCapture           bool     `mapstructure:"disable-assets-capture"`
	HQ                             bool     // Special field to check if HQ is enabled depending on the command called
	HQContinuousPull               bool     `mapstructure:"hq-continuous-pull"`
	HQRateLimitSendBack            bool     `mapstructure:"hq-rate-limiting-send-back"`
	NoStdoutLogging                bool     `mapstructure:"no-stdout-log"`
	NoBatchWriteWAL                bool     `mapstructure:"ultrasafe-queue"`
	Handover                       bool     `mapstructure:"handover"`

	// Network
	Proxy              string   `mapstructure:"proxy"`
	DomainsBypassProxy []string `mapstructure:"bypass-proxy"`
	RandomLocalIP      bool     `mapstructure:"random-local-ip"`
	DisableIPv4        bool     `mapstructure:"disable-ipv4"`
	DisableIPv6        bool     `mapstructure:"disable-ipv6"`
	IPv6AnyIP          bool     `mapstructure:"ipv6-anyip"`

	// Dependencies
	NoYTDLP   bool   `mapstructure:"no-ytdlp"`
	YTDLPPath string `mapstructure:"ytdlp-path"`
}

// Crawl define the parameters of a crawl process
type Crawl struct {
	*sync.Mutex
	StartTime time.Time
	// SeedList  []queue.Item
	// Paused    *utils.TAtomBool
	// Finished  *utils.TAtomBool
	LiveStats bool

	// Logger
	Log *log.Logger

	// Queue (ex-frontier)
	// Queue        *queue.PersistentGroupedQueue
	// Seencheck    *seencheck.Seencheck
	UseSeencheck bool
	UseHandover  bool
	UseCommit    bool

	// Worker pool
	// Workers *WorkerPool

	// Crawl settings
	MaxConcurrentAssets            int
	Client                         *warc.CustomHTTPClient
	ClientProxied                  *warc.CustomHTTPClient
	DisabledHTMLTags               []string
	ExcludedHosts                  []string
	IncludedHosts                  []string
	IncludedStrings                []string
	ExcludedStrings                []string
	UserAgent                      string
	Job                            string
	JobPath                        string
	MaxHops                        uint8
	MaxRetry                       uint8
	MaxRedirect                    uint8
	HTTPTimeout                    int
	MaxConcurrentRequestsPerDomain int
	RateLimitDelay                 int
	CrawlTimeLimit                 int
	MaxCrawlTimeLimit              int
	DisableAssetsCapture           bool
	CaptureAlternatePages          bool
	DomainsCrawl                   bool
	Headless                       bool
	MinSpaceRequired               int

	// Cookie-related settings
	CookieFile  string
	KeepCookies bool
	CookieJar   http.CookieJar

	// Network settings
	Proxy         string
	BypassProxy   []string
	RandomLocalIP bool
	DisableIPv4   bool
	DisableIPv6   bool
	IPv6AnyIP     bool

	// API settings
	API        bool
	APIPort    string
	Prometheus bool
	// PrometheusMetrics *PrometheusMetrics

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
	WARCDedupeSize     int
	WARCSize           int
	DisableLocalDedupe bool
	CertValidation     bool
	WARCCustomCookie   string

	// Crawl HQ settings
	UseHQ              bool
	HQAddress          string
	HQProject          string
	HQKey              string
	HQSecret           string
	HQStrategy         string
	HQBatchConcurrency int
	HQBatchSize        int
	HQContinuousPull   bool
	HQClient           *gocrawlhq.Client
	HQConsumerState    string
	// HQFinishedChannel      chan *queue.Item
	// HQProducerChannel      chan *queue.Item
	HQChannelsWg           *sync.WaitGroup
	HQRateLimitingSendBack bool

	// Dependencies
	NoYTDLP   bool
	YTDLPPath string
}

var (
	config *Config
	once   sync.Once
)

// InitConfig initializes the configuration
// Flags -> Env -> Config file -> Consul config
// Latest has precedence over the rest
func InitConfig() error {
	var err error
	once.Do(func() {
		config = &Config{}

		// Check if a config file is provided via flag
		if configFile := viper.GetString("config-file"); configFile != "" {
			viper.SetConfigFile(configFile)
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			viper.AddConfigPath(home)
			viper.SetConfigType("yaml")
			viper.SetConfigName("zeno-config")
		}

		viper.SetEnvPrefix("ZENO")
		replacer := strings.NewReplacer("-", "_", ".", "_")
		viper.SetEnvKeyReplacer(replacer)
		viper.AutomaticEnv()

		if err = viper.ReadInConfig(); err == nil {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}

		if viper.GetBool("consul-config") && viper.GetString("consul-address") != "" {
			var consulAddress *url.URL
			consulAddress, err = url.Parse(viper.GetString("consul-address"))
			if err != nil {
				return
			}

			consulPath, consulFile := filepath.Split(viper.GetString("consul-path"))
			viper.AddRemoteProvider("consul", consulAddress.String(), consulPath)
			viper.SetConfigType(filepath.Ext(consulFile))
			viper.SetConfigName(strings.TrimSuffix(consulFile, filepath.Ext(consulFile)))

			if err = viper.ReadInConfig(); err == nil {
				fmt.Println("Using config file:", viper.ConfigFileUsed())
			}
		}

		// This function is used to bring logic to the flags when needed (e.g. live-stats)
		handleFlagsEdgeCases()

		// This function is used to handle flags aliases (e.g. hops -> max-hops)
		handleFlagsAliases()

		// Unmarshal the config into the Config struct
		err = viper.Unmarshal(config)
	})
	return err
}

// BindFlags binds the flags to the viper configuration
// This is needed because viper doesn't support same flag name accross multiple commands
// Details here: https://github.com/spf13/viper/issues/375#issuecomment-794668149
func BindFlags(flagSet *pflag.FlagSet) {
	flagSet.VisitAll(func(flag *pflag.Flag) {
		viper.BindPFlag(flag.Name, flag)
	})
}

// GetConfig returns the config struct
func GetConfig() *Config {
	cfg := config
	if cfg == nil {
		panic("Config not initialized. Call InitConfig() before accessing the config.")
	}
	return cfg
}

func GenerateCrawlConfig(config *Config) (*Crawl, error) {
	var c = new(Crawl)

	// Statistics counters
	c.CrawledSeeds = new(ratecounter.Counter)
	c.CrawledAssets = new(ratecounter.Counter)
	c.ActiveWorkers = new(ratecounter.Counter)
	c.URIsPerSecond = ratecounter.NewRateCounter(1 * time.Second)

	c.LiveStats = config.LiveStats

	// If the job name isn't specified, we generate a random name
	if config.Job == "" {
		if config.HQProject != "" {
			c.Job = config.HQProject
		} else {
			UUID, err := uuid.NewUUID()
			if err != nil {
				slog.Error("cmd/utils.go:InitCrawlWithCMD():uuid.NewUUID()", "error", err)
				return nil, err
			}

			c.Job = UUID.String()
		}
	} else {
		c.Job = config.Job
	}

	c.JobPath = path.Join("jobs", config.Job)

	// TODO
	// c.Workers = NewPool(uint(config.WorkersCount), time.Second*60, c)

	c.UseSeencheck = !config.DisableSeencheck
	c.HTTPTimeout = config.HTTPTimeout
	c.MaxConcurrentRequestsPerDomain = config.MaxConcurrentRequestsPerDomain
	c.RateLimitDelay = config.ConcurrentSleepLength
	c.CrawlTimeLimit = config.CrawlTimeLimit

	// Defaults --max-crawl-time-limit to 10% more than --crawl-time-limit
	if config.CrawlMaxTimeLimit == 0 && config.CrawlTimeLimit != 0 {
		c.MaxCrawlTimeLimit = config.CrawlTimeLimit + (config.CrawlTimeLimit / 10)
	} else {
		c.MaxCrawlTimeLimit = config.CrawlMaxTimeLimit
	}

	c.MaxRetry = config.MaxRetry
	c.MaxRedirect = config.MaxRedirect
	c.MaxHops = config.MaxHops
	c.DomainsCrawl = config.DomainsCrawl
	c.DisableAssetsCapture = config.DisableAssetsCapture
	c.DisabledHTMLTags = config.DisableHTMLTag

	// We exclude some hosts by default
	c.ExcludedHosts = utils.DedupeStrings(append(config.ExcludeHosts, "archive.org", "archive-it.org"))

	c.IncludedHosts = config.IncludeHosts
	c.CaptureAlternatePages = config.CaptureAlternatePages
	c.ExcludedStrings = config.ExcludeString
	c.IncludedStrings = config.IncludeString

	c.MinSpaceRequired = config.MinSpaceRequired

	// WARC settings
	c.WARCPrefix = config.WARCPrefix
	c.WARCOperator = config.WARCOperator

	if config.WARCTempDir != "" {
		c.WARCTempDir = config.WARCTempDir
	} else {
		c.WARCTempDir = path.Join(c.JobPath, "temp")
	}

	c.CDXDedupeServer = config.CDXDedupeServer
	c.DisableLocalDedupe = config.DisableLocalDedupe
	c.CertValidation = config.CertValidation
	c.WARCFullOnDisk = config.WARCOnDisk
	c.WARCPoolSize = config.WARCPoolSize
	c.WARCDedupeSize = config.WARCDedupeSize
	c.WARCCustomCookie = config.CDXCookie
	c.WARCSize = config.WARCSize

	c.API = config.API
	c.APIPort = config.APIPort

	// If Prometheus is specified, then we make sure
	// c.API is true
	c.Prometheus = config.Prometheus
	if c.Prometheus {
		c.API = true
		// TODO: Implement Prometheus metrics
		// c.PrometheusMetrics = &PrometheusMetrics{}
		// c.PrometheusMetrics.Prefix = config.PrometheusPrefix
	}

	// Dependencies
	c.NoYTDLP = config.NoYTDLP
	c.YTDLPPath = config.YTDLPPath

	if config.UserAgent != "Zeno" {
		c.UserAgent = config.UserAgent
	} else {
		version := getVersion()

		// If Version is a commit hash, we only take the first 7 characters
		if len(version.Version) >= 40 {
			version.Version = version.Version[:7]
		}

		c.UserAgent = "Mozilla/5.0 (compatible; archive.org_bot +http://archive.org/details/archive.org_bot) Zeno/" + version.Version + " warc/" + version.WarcVersion
		slog.Info("User-Agent set to", "user-agent", c.UserAgent)
	}
	c.Headless = config.Headless

	c.CookieFile = config.Cookies
	c.KeepCookies = config.KeepCookies

	// Network settings
	c.Proxy = config.Proxy
	c.BypassProxy = config.DomainsBypassProxy
	c.RandomLocalIP = config.RandomLocalIP

	if c.RandomLocalIP {
		slog.Warn("Random local IP is enabled")
	}

	c.DisableIPv4 = config.DisableIPv4
	c.DisableIPv6 = config.DisableIPv6
	c.IPv6AnyIP = config.IPv6AnyIP

	if c.DisableIPv4 && c.DisableIPv6 {
		slog.Error("Both IPv4 and IPv6 are disabled, at least one of them must be enabled.")
		os.Exit(1)
	} else if c.DisableIPv4 {
		slog.Info("IPv4 is disabled")
	} else if c.DisableIPv6 {
		slog.Info("IPv6 is disabled")
	}

	// Crawl HQ settings
	c.UseHQ = config.HQ
	c.HQProject = config.HQProject
	c.HQAddress = config.HQAddress
	c.HQKey = config.HQKey
	c.HQSecret = config.HQSecret
	c.HQStrategy = config.HQStrategy
	c.HQBatchSize = int(config.HQBatchSize)
	c.HQBatchConcurrency = config.HQBatchConcurrency
	c.HQContinuousPull = config.HQContinuousPull
	c.HQRateLimitingSendBack = config.HQRateLimitSendBack

	// Handover mechanism
	c.UseHandover = config.Handover

	c.UseCommit = !config.NoBatchWriteWAL

	return c, nil
}

func handleFlagsEdgeCases() {
	if viper.GetBool("live-stats") {
		// If live-stats is true, set no-stdout-log to true
		viper.Set("no-stdout-log", true)
	}

	if viper.GetBool("prometheus") {
		// If prometheus is true, set no-stdout-log to true
		viper.Set("api", true)
	}
}

func handleFlagsAliases() {
	// For each flag we want to alias, we check if the original flag is at default and if the alias is not
	// If so, we set the original flag to the value of the alias

	if viper.GetUint("hops") != 0 && viper.GetUint("max-hops") == 0 {
		viper.Set("max-hops", viper.GetUint("hops"))
	}

	if viper.GetInt("ca") != 8 && viper.GetInt("max-concurrent-assets") == 8 {
		viper.Set("max-concurrent-assets", viper.GetInt("ca"))
	}

	if viper.GetInt("msr") != 20 && viper.GetInt("min-space-required") == 20 {
		viper.Set("min-space-required", viper.GetInt("msr"))
	}
}
