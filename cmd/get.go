/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"net/url"

	"github.com/CorentinB/Zeno/pkg/crawl"
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

var (
	maxHops  uint8
	logDebug bool
	logJSON  bool
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get [url]",
	Short: "Archive a single URL or website",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := initLogging(cmd)
		if err != nil {
			log.Fatal("Unable to parse arguments")
		}

		// Parse some arguments
		maxHops, err := cmd.Flags().GetUint8("max-hops")
		if err != nil {
			log.Fatal("Unable to parse --max-hops")
		}

		// Validate input URL
		URL, err := url.ParseRequestURI(args[0])
		if err != nil {
			log.WithFields(log.Fields{
				"url": args[0],
			}).Fatal("This is not a valid URL")
		}

		// Initialize crawl
		crawl := crawl.Create()
		crawl.MaxHops = maxHops
		crawl.Origin = URL
		crawl.Log = log.WithFields(log.Fields{
			"crawl": crawl,
		})

		crawl.Log.Info("Crawl starting")

		// Start crawl
		err = crawl.Start()
		if err != nil {
			log.WithFields(log.Fields{
				"crawl": crawl,
				"error": err,
			}).Fatal("Crawl exited due to error")
		}

		crawl.Log.Info("Crawl finished")
	},
}

func initLogging(cmd *cobra.Command) (err error) {
	// Log as JSON instead of the default ASCII formatter.
	logJSON, err = cmd.Flags().GetBool("json")
	if err != nil {
		return err
	}
	if logJSON {
		log.SetFormatter(&log.JSONFormatter{})
	}

	// Turn on debug mode
	logDebug, err = cmd.Flags().GetBool("debug")
	if err != nil {
		return err
	}
	if logDebug {
		log.SetLevel(log.DebugLevel)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(getCmd)

	// Log flags
	getCmd.PersistentFlags().Bool("debug", false, "Turn on debug mode")
	getCmd.PersistentFlags().Bool("json", false, "Turn on JSON logging")

	// Crawl flags
	getCmd.PersistentFlags().Uint8("max-hops", 0, "Maximum number of hops")
}
