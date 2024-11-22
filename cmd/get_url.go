package cmd

import (
	"fmt"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/spf13/cobra"
)

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
		for _, URL := range args {
			config.Get().InputSeeds = append(config.Get().InputSeeds, URL)
		}

		return config.GenerateCrawlConfig()
	},
}
