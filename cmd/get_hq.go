package cmd

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/grafana/pyroscope-go"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler"
	"github.com/internetarchive/Zeno/internal/pkg/ui"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
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
			runtime.SetMutexProfileFraction(5)
			runtime.SetBlockProfileRate(5)

			// Get the hostname via env or via command
			hostname, err := os.Hostname()
			if err != nil {
				return fmt.Errorf("error getting hostname for Pyroscope: %w", err)
			}

			Version := utils.GetVersion()

			_, err = pyroscope.Start(pyroscope.Config{
				ApplicationName: "zeno",
				ServerAddress:   cfg.PyroscopeAddress,
				Logger:          nil,
				Tags:            map[string]string{"hostname": hostname, "job": cfg.Job, "version": Version.Version, "goVersion": Version.GoVersion, "uuid": uuid.New().String()[:5]},
				UploadRate:      15 * time.Second,
				ProfileTypes: []pyroscope.ProfileType{
					pyroscope.ProfileCPU,
					pyroscope.ProfileAllocObjects,
					pyroscope.ProfileAllocSpace,
					pyroscope.ProfileInuseObjects,
					pyroscope.ProfileInuseSpace,
					pyroscope.ProfileGoroutines,
					pyroscope.ProfileMutexCount,
					pyroscope.ProfileMutexDuration,
					pyroscope.ProfileBlockCount,
					pyroscope.ProfileBlockDuration,
				},
			})

			if err != nil {
				panic(fmt.Errorf("error starting pyroscope: %w", err))
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
