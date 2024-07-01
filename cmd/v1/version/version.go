package version

import (
	"github.com/internetarchive/Zeno/cmd/v1"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/urfave/cli/v2"
)

func init() {
	cmd.RegisterCommand(
		cli.Command{
			Name:   "version",
			Usage:  "Show the version number.",
			Action: cmdVersion,
			Subcommands: []*cli.Command{
				newShowDepsCmd(),
			},
		})
}

func cmdVersion(c *cli.Context) error {
	version := utils.GetVersion()

	println("Zeno", version.Version)
	println("- go/version:", version.GoVersion)

	return nil
}
