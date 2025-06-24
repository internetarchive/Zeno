package cmd

import (
	"fmt"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler"
	"github.com/internetarchive/Zeno/internal/pkg/ui"
	"github.com/spf13/cobra"
)

var getURLCmd = &cobra.Command{
	Use:   "url [URL...]",
	Short: "Archive given URLs",
	Args:  cobra.MinimumNArgs(1),
	PreRunE: func(_ *cobra.Command, args []string) error {
		if cfg == nil {
			return fmt.Errorf("viper config is nil")
		}

		if len(args) == 0 {
			return fmt.Errorf("no URLs provided")
		}

		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		for _, URL := range args {
			config.Get().InputSeeds = append(config.Get().InputSeeds, URL)
		}

		err := config.GenerateCrawlConfig()
		if err != nil {
			return err
		}

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
