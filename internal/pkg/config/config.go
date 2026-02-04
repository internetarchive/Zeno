package config

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/domainscrawl"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	warc "github.com/internetarchive/gowarc"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config holds all configuration for our program, parsed from various sources
// The `mapstructure` tags are used to map the fields to the viper configuration
type Config struct {
	// Context for managing goroutine cancellation
	ctx       context.Context
	cancel    context.CancelFunc
	waitGroup sync.WaitGroup

	// Atomic flag to track if cancellation is requested
	cancellationRequested int32

	Job           string `mapstructure:"job"`
	JobPrometheus string
	JobPath       string

	// UseSeencheck exists just for convenience of not checking
	// !DisableSeencheck in the rest of the code, to make the code clearer
	DisableSeencheck bool `mapstructure:"disable-seencheck"`
	UseSeencheck     bool

	UserAgent                       string        `mapstructure:"user-agent"`
	Cookies                         string        `mapstructure:"cookies"`
	WARCPrefix                      string        `mapstructure:"warc-prefix"`
	WARCOperator                    string        `mapstructure:"warc-operator"`
	WARCTempDir                     string        `mapstructure:"warc-temp-dir"`
	WARCSize                        int           `mapstructure:"warc-size"`
	WARCOnDisk                      bool          `mapstructure:"warc-on-disk"`
	WARCPoolSize                    int           `mapstructure:"warc-pool-size"`
	WARCQueueSize                   int           `mapstructure:"warc-queue-size"`
	WARCDedupeSize                  int           `mapstructure:"warc-dedupe-size"`
	WARCDedupeCacheSize             int           `mapstructure:"warc-dedupe-cache-size"`
	WARCWriteAsync                  bool          `mapstructure:"async-warc-write"`
	WARCDiscardStatus               []int         `mapstructure:"warc-discard-status"`
	WARCDigestAlgorithm             string        `mapstructure:"warc-digest-algorithm"`
	CDXDedupeServer                 string        `mapstructure:"warc-cdx-dedupe-server"`
	CDXCookie                       string        `mapstructure:"warc-cdx-cookie"`
	DoppelgangerDedupeServer        string        `mapstructure:"warc-doppelganger-dedupe-server"`
	HQAddress                       string        `mapstructure:"hq-address"`
	HQKey                           string        `mapstructure:"hq-key"`
	HQSecret                        string        `mapstructure:"hq-secret"`
	HQProject                       string        `mapstructure:"hq-project"`
	HQTimeout                       int           `mapstructure:"hq-timeout"`
	HQBatchSize                     int           `mapstructure:"hq-batch-size"`
	HQBatchConcurrency              int           `mapstructure:"hq-batch-concurrency"`
	DisableHTMLTag                  []string      `mapstructure:"disable-html-tag"`
	ExcludeHosts                    []string      `mapstructure:"exclude-host"`
	IncludeHosts                    []string      `mapstructure:"include-host"`
	IncludeString                   []string      `mapstructure:"include-string"`
	ExcludeString                   []string      `mapstructure:"exclude-string"`
	ExclusionFile                   []string      `mapstructure:"exclusion-file"`
	ExclusionFileLiveReload         bool          `mapstructure:"exclusion-file-live-reload"`
	ExclusionFileLiveReloadInterval time.Duration `mapstructure:"exclusion-file-live-reload-interval"`
	WorkersCount                    int           `mapstructure:"workers"`
	MaxConcurrentAssets             int           `mapstructure:"max-concurrent-assets"`
	MaxHops                         int           `mapstructure:"max-hops"`
	MaxRedirect                     int           `mapstructure:"max-redirect"`
	MaxCSSJump                      int           `mapstructure:"max-css-jump"`
	MaxRetry                        int           `mapstructure:"max-retry"`
	MaxContentLengthMiB             int           `mapstructure:"max-content-length"`
	MaxOutlinks                     int           `mapstructure:"max-outlinks"`
	HTTPTimeout                     time.Duration `mapstructure:"http-timeout"`
	ConnReadDeadline                time.Duration `mapstructure:"conn-read-deadline"`
	CrawlTimeLimit                  int           `mapstructure:"crawl-time-limit"`
	CrawlMaxTimeLimit               int           `mapstructure:"crawl-max-time-limit"`
	MinSpaceRequired                float64       `mapstructure:"min-space-required"`
	DomainsCrawl                    []string      `mapstructure:"domains-crawl"`
	DomainsCrawlFile                []string      `mapstructure:"domains-crawl-file"`
	CaptureAlternatePages           bool          `mapstructure:"capture-alternate-pages"`
	StrictRegex                     bool          `mapstructure:"strict-regex"`
	DisableLocalDedupe              bool          `mapstructure:"disable-local-dedupe"`
	CertValidation                  bool          `mapstructure:"cert-validation"`
	DisableAssetsCapture            bool          `mapstructure:"disable-assets-capture"`
	UseHQ                           bool          // Special field to check if HQ is enabled depending on the command called

	// Headless
	Headless                 bool     `mapstructure:"headless"`
	HeadlessHeadful          bool     `mapstructure:"headless-headful"`
	HeadlessTrace            bool     `mapstructure:"headless-trace"`
	HeadlessChromiumRevision int      `mapstructure:"headless-chromium-revision"`
	HeadlessChromiumBin      string   `mapstructure:"headless-chromium-bin"`
	HeadlessDevTools         bool     `mapstructure:"headless-dev-tools"`
	HeadlessStealth          bool     `mapstructure:"headless-stealth"`
	HeadlessUserMode         bool     `mapstructure:"headless-user-mode"`
	HeadlessUserDataDir      string   `mapstructure:"headless-user-data-dir"`
	HeadlessAllowedMethods   []string `mapstructure:"headless-allowed-methods"`

	HeadlessPageTimeout       time.Duration `mapstructure:"headless-page-timeout"`
	HeadlessPageLoadTimeout   time.Duration `mapstructure:"headless-page-load-timeout"`
	HeadlessPagePostLoadDelay time.Duration `mapstructure:"headless-page-post-load-delay"`

	HeadlessBehaviors       []string      `mapstructure:"headless-behaviors"`
	HeadlessBehaviorTimeout time.Duration `mapstructure:"headless-behavior-timeout"`

	// Network
	Proxy         string `mapstructure:"proxy"`
	RandomLocalIP bool   `mapstructure:"random-local-ip"`
	DisableIPv4   bool   `mapstructure:"disable-ipv4"`
	DisableIPv6   bool   `mapstructure:"disable-ipv6"`
	IPv6AnyIP     bool   `mapstructure:"ipv6-anyip"`

	// Rate limiting
	DisableRateLimit          bool          `mapstructure:"disable-rate-limit"`
	RateLimitCapacity         float64       `mapstructure:"rate-limit-capacity"`
	RateLimitRefillRate       float64       `mapstructure:"rate-limit-refill-rate"`
	RateLimitCleanupFrequency time.Duration `mapstructure:"rate-limit-cleanup-frequency"`

	// Logging
	NoStdoutLogging  bool   `mapstructure:"no-stdout-log"`
	NoStderrLogging  bool   `mapstructure:"no-stderr-log"`
	NoColorLogging   bool   `mapstructure:"no-color-logs"`
	NoFileLogging    bool   `mapstructure:"no-log-file"`
	E2ELogging       bool   `mapstructure:"log-e2e"`
	E2ELevel         string `mapstructure:"log-e2e-level"`
	StdoutLogLevel   string `mapstructure:"log-level"`
	TUI              bool   `mapstructure:"tui"`
	TUILogLevel      string `mapstructure:"tui-log-level"`
	LogFileLevel     string `mapstructure:"log-file-level"`
	LogFileOutputDir string `mapstructure:"log-file-output-dir"`
	LogFilePrefix    string `mapstructure:"log-file-prefix"`
	LogFileRotation  string `mapstructure:"log-file-rotation"`

	// Profiling
	PyroscopeAddress string `mapstructure:"pyroscope-address"`
	SentryDSN        string `mapstructure:"sentry-dsn"`

	// API
	APIPort int  `mapstructure:"api-port"`
	API     bool `mapstructure:"api"`

	// Prometheus and metrics
	Prometheus       bool   `mapstructure:"prometheus"`
	PrometheusPrefix string `mapstructure:"prometheus-prefix"`

	// Consul
	ConsulAddress      string   `mapstructure:"consul-address"`
	ConsulPort         string   `mapstructure:"consul-port"`
	ConsulACLToken     string   `mapstructure:"consul-acl-token"`
	ConsulRegister     bool     `mapstructure:"consul-register"`
	ConsulRegisterTags []string `mapstructure:"consul-register-tags"`

	InputSeeds       []string     // Special field to store the input URLs
	exclusionRegexes atomic.Value // Special field to store the compiled exclusion regex (from --exclusion-file)
}

var (
	config *Config
	once   sync.Once
)

// Add this method to set the context on the package's config struct
func (c *Config) SetContext(ctx context.Context) {
	if ctx == nil {
		// Create a new context with cancel if none is provided
		c.ctx, c.cancel = context.WithCancel(context.Background())
	} else {
		// Use the provided context
		c.ctx, c.cancel = context.WithCancel(ctx)
	}
}

// Add this method to cancel the package's context
func (c *Config) Cancel() {
	if !atomic.CompareAndSwapInt32(&c.cancellationRequested, 0, 1) {
		return // Already cancelled
	}
	if c.cancel != nil {
		c.cancel()
		c.waitGroup.Wait()
	}
}

// Get returns the config struct
func Get() *Config {
	return config
}

// Useful for testing
func Set(cfg *Config) {
	config = cfg
}

// InitConfig initializes the configuration
// Flags -> Env -> Config file -> Consul config
// Latest has precedence over the rest
func InitConfig() error {
	var err error
	once.Do(func() {
		config = &Config{}
		config.SetContext(context.Background())

		// Check if a config file is provided via flag
		configFileProvided := viper.GetString("config-file") != ""
		if configFile := viper.GetString("config-file"); configFile != "" {
			viper.SetConfigFile(configFile)
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return
			}

			viper.AddConfigPath(home)
			viper.SetConfigType("yaml")
			viper.SetConfigName("zeno-config")
		}

		viper.SetEnvPrefix("ZENO")
		replacer := strings.NewReplacer("-", "_", ".", "_")
		viper.SetEnvKeyReplacer(replacer)
		viper.AutomaticEnv()

		err = viper.ReadInConfig()
		if err != nil {
			if configFileProvided {
				// User explicitly provided a config file, any error should be reported
				err = fmt.Errorf("error reading config file: %w", err)
				return
			} else {
				// Using default config file location
				// Only report errors for parsing issues, not for file not found
				if _, isNotFoundError := err.(viper.ConfigFileNotFoundError); !isNotFoundError {
					// Config file exists but has errors (e.g., invalid YAML)
					err = fmt.Errorf("error reading config file: %w", err)
					return
				}
				// Config file doesn't exist at default location, which is OK
				err = nil // Clear the error since file not found is OK for default config
			}
		} else {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}

		if viper.GetBool("consul-config") && viper.GetString("consul-address") != "" {
			var consulAddress *url.URL
			consulAddress, err := url.Parse(viper.GetString("consul-address"))
			if err != nil {
				return
			}

			consulPath, consulFile := filepath.Split(viper.GetString("consul-path"))
			viper.AddRemoteProvider("consul", consulAddress.String(), consulPath)
			viper.SetConfigType(filepath.Ext(consulFile))
			viper.SetConfigName(strings.TrimSuffix(consulFile, filepath.Ext(consulFile)))

			if err := viper.ReadInConfig(); err == nil {
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

func GenerateCrawlConfig() error {
	// If the job name isn't specified, we generate a random name
	if config.Job == "" {
		if config.HQProject != "" {
			config.Job = config.HQProject
		} else {
			UUID, err := uuid.NewUUID()
			if err != nil {
				slog.Error("cmd/utils.go:InitCrawlWithCMD():uuid.NewUUID()", "error", err)
				return err
			}

			config.Job = UUID.String()
		}
	}

	// Prometheus syntax does not play nice with hyphens
	config.JobPrometheus = strings.ReplaceAll(config.Job, "-", "")
	config.JobPath = path.Join("jobs", config.Job)
	config.UseSeencheck = !config.DisableSeencheck

	// Defaults --max-crawl-time-limit to 10% more than --crawl-time-limit
	if config.CrawlMaxTimeLimit == 0 && config.CrawlTimeLimit != 0 {
		config.CrawlMaxTimeLimit = config.CrawlTimeLimit + (config.CrawlTimeLimit / 10)
	}

	// We exclude some hosts by default
	config.ExcludeHosts = utils.DedupeStrings(append(config.ExcludeHosts, "archive.org", "archive-it.org"))

	if config.WARCTempDir == "" {
		config.WARCTempDir = path.Join(config.JobPath, "temp")
	}

	// Verify that the digest is supported
	if ok := warc.IsDigestSupported(config.WARCDigestAlgorithm); !ok {
		return fmt.Errorf("digest algorithm %s is not supported", config.WARCDigestAlgorithm)
	}

	if config.UserAgent == "" {
		version := utils.GetVersion()

		// If Version is a commit hash, we only take the first 7 characters
		if len(version.Version) >= 40 {
			version.Version = version.Version[:7]
		}

		config.UserAgent = "Mozilla/5.0 (compatible; archive.org_bot +http://archive.org/details/archive.org_bot) Zeno/" + version.Version + " warc/" + version.WarcVersion
		slog.Info("User-Agent set to", "user-agent", config.UserAgent)
	}

	if config.MaxContentLengthMiB > 0 {
		slog.Info("max content length is set, payload over X MiB would be discarded", "X", config.MaxContentLengthMiB)
	}

	if config.MaxOutlinks > 0 {
		slog.Info("max outlinks is set, only the first X outlinks will be processed", "X", config.MaxOutlinks)
	}

	if config.RandomLocalIP {
		slog.Warn("random local IP is enabled")
	}

	if config.DisableIPv4 && config.DisableIPv6 {
		return fmt.Errorf("both IPv4 and IPv6 are disabled, at least one of them must be enabled.")
	} else if config.DisableIPv4 {
		slog.Info("IPv4 is disabled")
	} else if config.DisableIPv6 {
		slog.Info("IPv6 is disabled")
	}

	config.exclusionRegexes.Store([]*regexp.Regexp(nil))
	if len(config.ExclusionFile) > 0 {
		var exclusions []*regexp.Regexp

		for _, file := range config.ExclusionFile {
			newExclusions, err := config.loadExclusions(file)
			if err != nil {
				return err
			}

			exclusions = append(exclusions, newExclusions...)
		}

		config.setExclusionRegexes(exclusions)

		if config.ExclusionFileLiveReload {
			config.waitGroup.Go(config.exclusionFileLiveReloader)
		}
	}

	if len(config.DomainsCrawl) > 0 || len(config.DomainsCrawlFile) > 0 {
		slog.Info("domains crawl enabled", "domains/regex", config.DomainsCrawl)
		err := domainscrawl.AddElements(config.DomainsCrawl, config.DomainsCrawlFile)
		if err != nil {
			panic(err)
		}
	}

	// In CI/CD, set a low threshold for testing purposes
	if config.MinSpaceRequired == 0 && os.Getenv("CI") == "true" {
		config.MinSpaceRequired = 1 // 1 GiB
	}

	return nil
}

func handleFlagsEdgeCases() {
	if viper.GetBool("tui") {
		// If live-stats is true, set no-stdout-log to true
		viper.Set("no-stdout-log", true)
		viper.Set("no-stderr-log", true)
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

	if viper.GetInt("ca") != 1 && viper.GetInt("max-concurrent-assets") == 1 {
		viper.Set("max-concurrent-assets", viper.GetInt("ca"))
	}

	if viper.GetInt("msr") != 20 && viper.GetInt("min-space-required") == 20 {
		viper.Set("min-space-required", viper.GetInt("msr"))
	}
}
