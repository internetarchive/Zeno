package version

import (
	"github.com/CorentinB/Zeno/cmd"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
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
