package cmd

import (
	"fmt"

	"github.com/internetarchive/Zeno/v2/internal/pkg/config"
	"github.com/internetarchive/Zeno/v2/internal/pkg/controler"
	"github.com/internetarchive/Zeno/v2/internal/pkg/ui"
	"github.com/spf13/cobra"
)

var getHQ4Cmd = &cobra.Command{
	Use:   "hq4",
	Short: "Start crawling with the crawl HQv4 connector.",
	PreRunE: func(_ *cobra.Command, _ []string) error {
		if cfg == nil {
			return fmt.Errorf("viper config is nil")
		}

		err := config.GenerateCrawlConfig()
		if err != nil {
			return err
		}

		cfg.UseHQ4 = true

		if cfg.PyroscopeAddress != "" {
			err = startPyroscope()
			if err != nil {
				return err
			}
		}

		if cfg.SentryDSN != "" {
			err = startSentry()
			if err != nil {
				return err
			}
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

func getHQ4CmdFlags(getHQ4Cmd *cobra.Command) {
	// below are flags carried over from HQv3
	//
	// general required flags
	getHQ4Cmd.PersistentFlags().String("hq-address", "", "Crawl HQ address.")
	getHQ4Cmd.PersistentFlags().String("hq-key", "", "Crawl HQ key.")
	getHQ4Cmd.PersistentFlags().String("hq-secret", "", "Crawl HQ secret.")

	getHQCmd.MarkPersistentFlagRequired("hq-address")
	getHQCmd.MarkPersistentFlagRequired("hq-key")
	getHQCmd.MarkPersistentFlagRequired("hq-secret")
	// the flag 'hq-project' used in HQv3 has been abandoned in favor of the flag 'hq4-project-uuid'
	//
	// optional flags
	getHQ4Cmd.PersistentFlags().Int("hq-timeout", 5, "Crawl HQ HTTP Client default timeout")
	getHQ4Cmd.PersistentFlags().Int("hq-batch-size", 500, "Crawl HQ feeding batch size.")
	getHQ4Cmd.PersistentFlags().Int("hq-batch-concurrency", 1, "Number of concurrent requests to do to get the --hq-batch-size, if batch size is 300 and batch-concurrency is 10, 30 requests will be done concurrently.")
	getHQ4Cmd.PersistentFlags().Int("hq-seencheck-cache-size", 0, "Size of the local seencheck cache. When > 0, an in-memory otter cache is used to avoid sending duplicate seencheck requests to HQ.")
	getHQ4Cmd.PersistentFlags().String("hq-seencheck-url", "", "Alternative seencheck URL. When set, seencheck requests are sent to this URL instead of the default HQ seencheck endpoint.")
	getHQ4Cmd.PersistentFlags().Bool("hq-gzip-requests", false, "If turned on, requests to Crawl HQ will be GZIP compressed.")
	getHQ4Cmd.PersistentFlags().Bool("hq-rate-limiting-send-back", false, "If turned on, the crawler will send back URLs that hit a rate limit to crawl HQ.")

	// below are flags specific to HQv4
	getHQ4Cmd.PersistentFlags().String("hq4-project-uuid", "", "Crawl HQ address.")
	getHQ4Cmd.PersistentFlags().String("hq4-routing-key", "", "Crawl HQ address.")
	getHQ4Cmd.PersistentFlags().String("hq4-rabbit-addr", "", "Crawl HQ address.")

	getHQ4Cmd.MarkPersistentFlagRequired("hq4-project-uuid")
	getHQ4Cmd.MarkPersistentFlagRequired("hq4-routing-key")
	getHQ4Cmd.MarkPersistentFlagRequired("hq4-rabbit-addr")
}
