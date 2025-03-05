package cmd

import (
	"fmt"

	"github.com/internetarchive/Zeno/internal/pkg/crawl"
	"github.com/internetarchive/Zeno/internal/pkg/queue"
	"github.com/spf13/cobra"
)

var (
	remoteFlag bool
)

var getListCmd = &cobra.Command{
	Use:   "list [FILE/URL]",
	Short: "Start crawling with a seed list from a local file or remote URL",
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
				}).Error("'get list' exited due to error")
			}
			return err
		}

		// Determine seed list source
		input := args[0]
		var seedList []queue.Item

		// Choose loading method based on flag
		if remoteFlag {
			seedList, err = queue.FetchRemoteList(input)
		} else {
			seedList, err = queue.FileToItems(input)
		}

		// Handle loading errors
		if err != nil {
			errorSource := map[bool]string{true: "remote", false: "local"}[remoteFlag]
			crawl.Log.WithFields(map[string]interface{}{
				"input": input,
				"err":   err.Error(),
			}).Error("Failed to read %s seed list", errorSource)
			return err
		}

		// Validate seed list
		if len(seedList) == 0 {
			crawl.Log.WithFields(map[string]interface{}{
				"input": input,
			}).Error("Seed list is empty")
			return fmt.Errorf("empty seed list")
		}

		crawl.SeedList = seedList
		crawl.Log.WithFields(map[string]interface{}{
			"input":      input,
			"seedsCount": len(crawl.SeedList),
			"source":     map[bool]string{true: "Remote", false: "Local"}[remoteFlag],
		}).Info("Seed list loaded")

		// Start crawl
		if err = crawl.Start(); err != nil {
			crawl.Log.WithFields(map[string]interface{}{
				"crawl": crawl,
				"err":   err.Error(),
			}).Error("Crawl exited due to error")
			return err
		}

		return nil
	},
}

func init() {
	getListCmd.Flags().BoolVar(&remoteFlag, "remote", false, "Treat input as a remote URL for seed list")
}