package cmd

import (
	"fmt"
	"net/url"

	"github.com/internetarchive/Zeno/internal/pkg/crawl"
	"github.com/internetarchive/Zeno/internal/pkg/frontier"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func getCMDs() *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Archive the web!",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				cmd.Help()
			}
		},
	}

	getCMDsFlags(getCmd)

	getCmd.AddCommand(getURLCmd)
	getCmd.AddCommand(getHQCmd)
	getCmd.AddCommand(getListCmd)

	return getCmd
}

func getCMDsFlags(getCmd *cobra.Command) {
	getCmd.PersistentFlags().String("user-agent", "Zeno", "User agent to use when requesting URLs.")
	getCmd.PersistentFlags().String("job", "", "Job name to use, will determine the path for the persistent queue, seencheck database, and WARC files.")
	getCmd.PersistentFlags().Int("workers", 1, "Number of concurrent workers to run.")
	getCmd.PersistentFlags().Int("max-concurrent-assets", 8, "Max number of concurrent assets to fetch PER worker. E.g. if you have 100 workers and this setting at 8, Zeno could do up to 800 concurrent requests at any time.")
	getCmd.PersistentFlags().Uint("max-hops", 0, "Maximum number of hops to execute.")
	getCmd.PersistentFlags().String("cookies", "", "File containing cookies that will be used for requests.")
	getCmd.PersistentFlags().Bool("keep-cookies", false, "Keep a global cookie jar")
	getCmd.PersistentFlags().Bool("headless", false, "Use headless browsers instead of standard GET requests.")
	getCmd.PersistentFlags().Bool("local-seencheck", false, "Simple local seencheck to avoid re-crawling of URIs.")
	getCmd.PersistentFlags().Bool("json", false, "Output logs in JSON")
	getCmd.PersistentFlags().Bool("debug", false, "")
	getCmd.PersistentFlags().Bool("api", false, "")
	getCmd.PersistentFlags().String("api-port", "9443", "Port to listen on for the API.")
	getCmd.PersistentFlags().Bool("prometheus", false, "Export metrics in Prometheus format. (implies --api)")
	getCmd.PersistentFlags().String("prometheus-prefix", "zeno:", "String used as a prefix for the exported Prometheus metrics.")
	getCmd.PersistentFlags().Int("max-redirect", 20, "Specifies the maximum number of redirections to follow for a resource.")
	getCmd.PersistentFlags().Int("max-retry", 20, "Number of retry if error happen when executing HTTP request.")
	getCmd.PersistentFlags().Int("http-timeout", 30, "Number of seconds to wait before timing out a request.")
	getCmd.PersistentFlags().Bool("domains-crawl", false, "If this is turned on, seeds will be treated as domains to crawl, therefore same-domain outlinks will be added to the queue as hop=0.")
	getCmd.PersistentFlags().StringSlice("disable-html-tag", []string{}, "Specify HTML tag to not extract assets from")
	getCmd.PersistentFlags().Bool("capture-alternate-pages", false, "If turned on, <link> HTML tags with \"alternate\" values for their \"rel\" attribute will be archived.")
	getCmd.PersistentFlags().StringSlice("exclude-host", []string{}, "Exclude a specific host from the crawl, note that it will not exclude the domain if it is encountered as an asset for another web page.")
	getCmd.PersistentFlags().StringSlice("include-host", []string{}, "Only crawl specific hosts, note that it will not include the domain if it is encountered as an asset for another web page.")
	getCmd.PersistentFlags().Int("max-concurrent-per-domain", 16, "Maximum number of concurrent requests per domain.")
	getCmd.PersistentFlags().Int("concurrent-sleep-length", 500, "Number of milliseconds to sleep when max concurrency per domain is reached.")
	getCmd.PersistentFlags().Int("crawl-time-limit", 0, "Number of seconds until the crawl will automatically set itself into the finished state.")
	getCmd.PersistentFlags().Int("crawl-max-time-limit", 0, "Number of seconds until the crawl will automatically panic itself. Default to crawl-time-limit + (crawl-time-limit / 10)")
	getCmd.PersistentFlags().StringSlice("exclude-string", []string{}, "Discard any (discovered) URLs containing this string.")
	getCmd.PersistentFlags().Bool("random-local-ip", false, "Use random local IP for requests. (will be ignored if a proxy is set)")
	getCmd.PersistentFlags().Int("min-space-required", 20, "Minimum space required in GB to continue the crawl.")

	// Proxy flags
	getCmd.PersistentFlags().String("proxy", "", "Proxy to use when requesting pages.")
	getCmd.PersistentFlags().StringSlice("bypass-proxy", []string{}, "Domains that should not be proxied.")

	// WARC flags
	getCmd.PersistentFlags().String("warc-prefix", "ZENO", "Prefix to use when naming the WARC files.")
	getCmd.PersistentFlags().String("warc-operator", "", "Contact informations of the crawl operator to write in the Warc-Info record in each WARC file.")
	getCmd.PersistentFlags().String("warc-cdx-dedupe-server", "", "Identify the server to use CDX deduplication. This also activates CDX deduplication on.")
	getCmd.PersistentFlags().Bool("warc-on-disk", false, "Do not use RAM to store payloads when recording traffic to WARCs, everything will happen on disk (usually used to reduce memory usage).")
	getCmd.PersistentFlags().Int("warc-pool-size", 1, "Number of concurrent WARC files to write.")
	getCmd.PersistentFlags().String("warc-temp-dir", "", "Custom directory to use for WARC temporary files.")
	getCmd.PersistentFlags().Bool("disable-local-dedupe", false, "Disable local URL agonistic deduplication.")
	getCmd.PersistentFlags().Bool("cert-validation", false, "Enables certificate validation on HTTPS requests.")
	getCmd.PersistentFlags().Bool("disable-assets-capture", false, "Disable assets capture.")
	getCmd.PersistentFlags().Int("warc-dedupe-size", 1024, "Minimum size to deduplicate WARC records with revisit records.")
	getCmd.PersistentFlags().String("cdx-cookie", "", "Pass custom cookie during CDX requests. Example: 'cdx_auth_token=test_value'")

	// Logging flags
	getCmd.PersistentFlags().Bool("live-stats", false, "Enable live stats but disable logging. (implies --no-stdout-log)")
	getCmd.PersistentFlags().String("log-file-output-dir", "./jobs/", "Directory to write log files to.")
	getCmd.PersistentFlags().String("es-url", "", "comma-separated ElasticSearch URL to use for indexing crawl logs.")
	getCmd.PersistentFlags().String("es-user", "", "ElasticSearch username to use for indexing crawl logs.")
	getCmd.PersistentFlags().String("es-password", "", "ElasticSearch password to use for indexing crawl logs.")
	getCmd.PersistentFlags().String("es-index-prefix", "zeno", "ElasticSearch index prefix to use for indexing crawl logs. Default is : `zeno`, without `-`")
}

var getURLCmd = &cobra.Command{
	Use:   "url [URL...]",
	Short: "Archive given URLs",
	Args:  cobra.MinimumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			return fmt.Errorf("viper config is nil")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Init crawl using the flags provided
		crawl, err := crawl.GenerateCrawlConfig(cfg)
		if err != nil {
			if crawl != nil && crawl.Log != nil {
				crawl.Log.WithFields(logrus.Fields{
					"crawl": crawl,
					"err":   err.Error(),
				}).Error("'get url' exited due to error")
			}
			return err
		}

		// Initialize initial seed list
		for _, arg := range args {
			input, err := url.Parse(arg)
			if err != nil {
				crawl.Log.WithFields(logrus.Fields{
					"input_url": arg,
					"err":       err.Error(),
				}).Error("given URL is not a valid input")
				return err
			}

			crawl.SeedList = append(crawl.SeedList, *frontier.NewItem(input, nil, "seed", 0, "", false))
		}

		// Start crawl
		err = crawl.Start()
		if err != nil {
			crawl.Log.WithFields(logrus.Fields{
				"crawl": crawl,
				"err":   err.Error(),
			}).Error("'get url' Crawl() exited due to error")
			return err
		}

		crawl.Log.Info("Crawl finished")
		return err
	},
}

var getHQCmd = &cobra.Command{
	Use:   "hq",
	Short: "Start crawling with the crawl HQ connector.",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			return fmt.Errorf("viper config is nil")
		}
		cfg.HQ = true
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Init crawl using the flags provided
		crawl, err := crawl.GenerateCrawlConfig(cfg)
		if err != nil {
			if crawl != nil && crawl.Log != nil {
				crawl.Log.WithFields(logrus.Fields{
					"crawl": crawl,
					"err":   err.Error(),
				}).Error("'get hq' exited due to error")
			}
			return err
		}

		// start crawl
		err = crawl.Start()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"crawl": crawl,
				"err":   err.Error(),
			}).Error("'get hq' Crawl() exited due to error")
			return err
		}

		return nil
	},
}

func getHQCmdFlags(getHQCmd *cobra.Command) {
	// Crawl HQ flags
	getHQCmd.PersistentFlags().String("hq-address", "", "Crawl HQ address.")
	getHQCmd.PersistentFlags().String("hq-key", "", "Crawl HQ key.")
	getHQCmd.PersistentFlags().String("hq-secret", "", "Crawl HQ secret.")
	getHQCmd.PersistentFlags().String("hq-project", "", "Crawl HQ project.")
	getHQCmd.PersistentFlags().Int64("hq-batch-size", 0, "Crawl HQ feeding batch size.")
	getHQCmd.PersistentFlags().Bool("hq-continuous-pull", false, "If turned on, the crawler will pull URLs from Crawl HQ continuously.")
	getHQCmd.PersistentFlags().String("hq-strategy", "lifo", "Crawl HQ feeding strategy.")
	getHQCmd.PersistentFlags().Bool("hq-rate-limiting-send-back", false, "If turned on, the crawler will send back URLs that hit a rate limit to crawl HQ.")
}

var getListCmd = &cobra.Command{
	Use:   "list [FILE]",
	Short: "Start crawling with a seed list",
	Args:  cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			return fmt.Errorf("viper config is nil")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Init crawl using the flags provided
		crawl, err := crawl.GenerateCrawlConfig(cfg)
		if err != nil {
			if crawl != nil && crawl.Log != nil {
				crawl.Log.WithFields(logrus.Fields{
					"crawl": crawl,
					"err":   err.Error(),
				}).Error("'get hq' exited due to error")
			}
			return err
		}

		// Initialize initial seed list
		crawl.SeedList, err = frontier.IsSeedList(args[0])
		if err != nil || len(crawl.SeedList) <= 0 {
			logrus.WithFields(logrus.Fields{
				"input": args[0],
				"err":   err.Error(),
			}).Error("This is not a valid input")
			return err
		}

		logrus.WithFields(logrus.Fields{
			"input":      args[0],
			"seedsCount": len(crawl.SeedList),
		}).Print("Seed list loaded")

		// Start crawl
		err = crawl.Start()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"crawl": crawl,
				"err":   err.Error(),
			}).Error("Crawl exited due to error")
			return err
		}

		return nil
	},
}
