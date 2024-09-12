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
	LogLevel                       string   `mapstructure:"log-level"`
	UserAgent                      string   `mapstructure:"user-agent"`
	Job                            string   `mapstructure:"job"`
	Cookies                        string   `mapstructure:"cookies"`
	APIPort                        string   `mapstructure:"api-port"`
	PrometheusPrefix               string   `mapstructure:"prometheus-prefix"`
	Proxy                          string   `mapstructure:"proxy"`
	WARCPrefix                     string   `mapstructure:"warc-prefix"`
	WARCOperator                   string   `mapstructure:"warc-operator"`
	CDXDedupeServer                string   `mapstructure:"warc-cdx-dedupe-server"`
	WARCTempDir                    string   `mapstructure:"warc-temp-dir"`
	CDXCookie                      string   `mapstructure:"cdx-cookie"`
	HQAddress                      string   `mapstructure:"hq-address"`
	HQKey                          string   `mapstructure:"hq-key"`
	HQSecret                       string   `mapstructure:"hq-secret"`
	HQProject                      string   `mapstructure:"hq-project"`
	HQStrategy                     string   `mapstructure:"hq-strategy"`
	LogFileOutputDir               string   `mapstructure:"log-file-output-dir"`
	ElasticSearchUsername          string   `mapstructure:"es-user"`
	ElasticSearchPassword          string   `mapstructure:"es-password"`
	ElasticSearchIndexPrefix       string   `mapstructure:"es-index-prefix"`
	DisableHTMLTag                 []string `mapstructure:"disable-html-tag"`
	ExcludeHosts                   []string `mapstructure:"exclude-host"`
	IncludeHosts                   []string `mapstructure:"include-host"`
	ExcludeString                  []string `mapstructure:"exclude-string"`
	DomainsBypassProxy             []string `mapstructure:"bypass-proxy"`
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
	HQBatchSize                    int64    `mapstructure:"hq-batch-size"`
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
	RandomLocalIP                  bool     `mapstructure:"random-local-ip"`
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

	// Dependencies
	NoYTDLP   bool   `mapstructure:"no-ytdlp"`
	YTDLPPath string `mapstructure:"ytdlp-path"`
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
