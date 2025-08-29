package cmd

import (
	"fmt"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler"
	"github.com/internetarchive/Zeno/internal/pkg/ui"
	"github.com/spf13/cobra"
)

var getHQCmd = &cobra.Command{
	Use:   "hq",
	Short: "Start crawling with the crawl HQ connector.",
	PreRunE: func(_ *cobra.Command, _ []string) error {
		if cfg == nil {
			return fmt.Errorf("viper config is nil")
		}

		err := config.GenerateCrawlConfig()
		if err != nil {
			return err
		}

		cfg.UseHQ = true

		if cfg.PyroscopeAddress != "" {
			err = startPyroscope()
			if err != nil {
				return err
			}
		}

		if cfg.SentryDSN != "" {
			startSentry()
		}

		return nil
	},
	RunE: func(_ *cobra.Command, _ []string) error {
		controler.Start()
		if config.Get().TUI {
			tui := ui.New()
			err := tui.Start()
			if err != nil {
				return fmt.Errorf("error starting TUI: %w", err)
			}
		} else {
			controler.WatchSignals()
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
	getHQCmd.PersistentFlags().Int("hq-batch-size", 500, "Crawl HQ feeding batch size.")
	getHQCmd.PersistentFlags().Int("hq-batch-concurrency", 1, "Number of concurrent requests to do to get the --hq-batch-size, if batch size is 300 and batch-concurrency is 10, 30 requests will be done concurrently.")
	getHQCmd.PersistentFlags().Bool("hq-rate-limiting-send-back", false, "If turned on, the crawler will send back URLs that hit a rate limit to crawl HQ.")

	getHQCmd.MarkPersistentFlagRequired("hq-address")
	getHQCmd.MarkPersistentFlagRequired("hq-key")
	getHQCmd.MarkPersistentFlagRequired("hq-secret")
	getHQCmd.MarkPersistentFlagRequired("hq-project")
}
