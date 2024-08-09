package cmd

import (
	"fmt"

	"github.com/internetarchive/Zeno/internal/crawl"
	"github.com/internetarchive/Zeno/internal/queue"
	"github.com/spf13/cobra"
)

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
				crawl.Log.WithFields(map[string]interface{}{
					"crawl": crawl,
					"err":   err.Error(),
				}).Error("'get hq' exited due to error")
			}
			return err
		}

		// Initialize initial seed list
		crawl.SeedList, err = queue.FileToItems(args[0])
		if err != nil || len(crawl.SeedList) <= 0 {
			crawl.Log.WithFields(map[string]interface{}{
				"input": args[0],
				"err":   err.Error(),
			}).Error("This is not a valid input")
			return err
		}

		crawl.Log.WithFields(map[string]interface{}{
			"input":      args[0],
			"seedsCount": len(crawl.SeedList),
		}).Info("Seed list loaded")

		// Start crawl
		err = crawl.Start()
		if err != nil {
			crawl.Log.WithFields(map[string]interface{}{
				"crawl": crawl,
				"err":   err.Error(),
			}).Error("Crawl exited due to error")
			return err
		}

		return nil
	},
}
