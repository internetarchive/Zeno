package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config holds all configuration for our program
type Config struct {
	// Global Flags
	LogLevel string `mapstructure:"log-level"`

	// Get flags (crawling flags)
	UserAgent                      string   `mapstructure:"user-agent"`
	Job                            string   `mapstructure:"job"`
	WorkersCount                   int      `mapstructure:"workers"`
	MaxConcurrentAssets            int      `mapstructure:"max-concurrent-assets"`
	MaxHops                        uint     `mapstructure:"max-hops"`
	Cookies                        string   `mapstructure:"cookies"`
	KeepCookies                    bool     `mapstructure:"keep-cookies"`
	Headless                       bool     `mapstructure:"headless"`
	LocalSeencheck                 bool     `mapstructure:"local-seencheck"`
	JSON                           bool     `mapstructure:"json"`
	Debug                          bool     `mapstructure:"debug"`
	LiveStats                      bool     `mapstructure:"live-stats"`
	API                            bool     `mapstructure:"api"`
	APIPort                        string   `mapstructure:"api-port"`
	Prometheus                     bool     `mapstructure:"prometheus"`
	PrometheusPrefix               string   `mapstructure:"prometheus-prefix"`
	MaxRedirect                    int      `mapstructure:"max-redirect"`
	MaxRetry                       int      `mapstructure:"max-retry"`
	HTTPTimeout                    int      `mapstructure:"http-timeout"`
	DomainsCrawl                   bool     `mapstructure:"domains-crawl"`
	DisableHTMLTag                 []string `mapstructure:"disable-html-tag"`
	CaptureAlternatePages          bool     `mapstructure:"capture-alternate-pages"`
	ExcludeHosts                   []string `mapstructure:"exclude-host"`
	IncludeHosts                   []string `mapstructure:"include-host"`
	MaxConcurrentRequestsPerDomain int      `mapstructure:"max-concurrent-per-domain"`
	ConcurrentSleepLength          int      `mapstructure:"concurrent-sleep-length"`
	CrawlTimeLimit                 int      `mapstructure:"crawl-time-limit"`
	CrawlMaxTimeLimit              int      `mapstructure:"crawl-max-time-limit"`
	ExcludeString                  []string `mapstructure:"exclude-string"`
	RandomLocalIP                  bool     `mapstructure:"random-local-ip"`

	// Get flags (Proxy flags)
	Proxy              string   `mapstructure:"proxy"`
	DomainsBypassProxy []string `mapstructure:"bypass-proxy"`

	// Get flags (WARC flags)
	WARCPrefix           string `mapstructure:"warc-prefix"`
	WARCOperator         string `mapstructure:"warc-operator"`
	CDXDedupeServer      string `mapstructure:"warc-cdx-dedupe-server"`
	WARCOnDisk           bool   `mapstructure:"warc-on-disk"`
	WARCPoolSize         int    `mapstructure:"warc-pool-size"`
	WARCTempDir          string `mapstructure:"warc-temp-dir"`
	DisableLocalDedupe   bool   `mapstructure:"disable-local-dedupe"`
	CertValidation       bool   `mapstructure:"cert-validation"`
	DisableAssetsCapture bool   `mapstructure:"disable-assets-capture"`
	WARCDedupeSize       int    `mapstructure:"warc-dedupe-size"`
	CDXCookie            string `mapstructure:"cdx-cookie"`

	// Get flags (Crawl HQ flags)
	HQ                  bool   // Special field to check if HQ is enabled depending on the command called
	HQAddress           string `mapstructure:"hq-address"`
	HQKey               string `mapstructure:"hq-key"`
	HQSecret            string `mapstructure:"hq-secret"`
	HQProject           string `mapstructure:"hq-project"`
	HQBatchSize         int64  `mapstructure:"hq-batch-size"`
	HQContinuousPull    bool   `mapstructure:"hq-continuous-pull"`
	HQStrategy          string `mapstructure:"hq-strategy"`
	HQRateLimitSendBack bool   `mapstructure:"hq-rate-limiting-send-back"`

	// Get flags (Logging flags)
	LogFileOutputDir         string   `mapstructure:"log-file-output-dir"`
	ElasticSearchURLs        []string `mapstructure:"es-url"`
	ElasticSearchUsername    string   `mapstructure:"es-user"`
	ElasticSearchPassword    string   `mapstructure:"es-password"`
	ElasticSearchIndexPrefix string   `mapstructure:"es-index-prefix"`
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
