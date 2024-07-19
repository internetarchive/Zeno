package cmd

import (
	"fmt"
	"net/url"

	"github.com/internetarchive/Zeno/internal/pkg/crawl"
	"github.com/internetarchive/Zeno/internal/pkg/frontier"
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
		// Init crawl using the flags provided
		crawl, err := crawl.GenerateCrawlConfig(cfg)
		if err != nil {
			if crawl != nil && crawl.Log != nil {
				crawl.Log.WithFields(map[string]interface{}{
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
				crawl.Log.WithFields(map[string]interface{}{
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
			crawl.Log.WithFields(map[string]interface{}{
				"crawl": crawl,
				"err":   err.Error(),
			}).Error("'get url' Crawl() exited due to error")
			return err
		}

		crawl.Log.Info("Crawl finished")
		return err
	},
}
