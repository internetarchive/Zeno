package cmd

import (
	"fmt"

	"github.com/internetarchive/Zeno/internal/pkg/crawl"
	"github.com/spf13/cobra"
)

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
				crawl.Log.WithFields(map[string]interface{}{
					"crawl": crawl,
					"err":   err.Error(),
				}).Error("'get hq' exited due to error")
			}
			return err
		}

		// start crawl
		err = crawl.Start()
		if err != nil {
			crawl.Log.WithFields(map[string]interface{}{
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
