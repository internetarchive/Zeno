package cmd

import (
	"fmt"
	"os"

	"github.com/internetarchive/Zeno/config/v2"
	"github.com/spf13/cobra"
)

var cfg *config.Config

var rootCmd = &cobra.Command{
	Use:   "Zeno",
	Short: "State-of-the-art web crawler 🔱",
	Long: `Zeno is a web crawler designed to operate wide crawls or to simply archive one web page.
Zeno's key concepts are: portability, performance, simplicity ; with an emphasis on performance.

Authors:
  Corentin Barreau <corentin@archive.org>
  Jake LaFountain <jakelf@archive.org>
  Thomas Foubert <thomas@archive.org>
`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize config here, after cobra has parsed command line flags
		if err := config.InitConfig(); err != nil {
			fmt.Printf("error initializing config: %s", err)
			os.Exit(1)
		}

		cfg = config.GetConfig()
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// Run the root command
func Run() error {
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Define flags and configuration settings
	rootCmd.PersistentFlags().String("log-level", "info", "stdout log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String("config", "", "config file (default is $HOME/zeno-config.yaml)")

	// Bind flags to viper
	config.BindFlags(rootCmd.Flags())

	addGetCMDs(rootCmd)

	return rootCmd.Execute()
}
