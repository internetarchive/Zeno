package cmd

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/CorentinB/Zeno/config"
)

var GlobalFlags = []cli.Flag{
	&cli.IntFlag{
		Name:        "workers",
		Aliases:     []string{"w"},
		Value:       1,
		Usage:       "Number of concurrent workers to run",
		Destination: &config.App.Flags.Workers,
	},
	&cli.UintFlag{
		Name:        "max-hops",
		Value:       0,
		Usage:       "Maximum number of hops to execute",
		Destination: &config.App.Flags.MaxHops,
	},
	&cli.BoolFlag{
		Name:        "headless",
		Usage:       "Use headless browsers instead of standard GET requests",
		Destination: &config.App.Flags.Headless,
	},
	&cli.BoolFlag{
		Name:        "seencheck",
		Usage:       "Simple seen check to avoid re-crawling of URIs",
		Destination: &config.App.Flags.Seencheck,
	},
	&cli.BoolFlag{
		Name:        "warc",
		Usage:       "Write all traffic in WARC files",
		Destination: &config.App.Flags.WARC,
	},
	&cli.BoolFlag{
		Name:        "json",
		Usage:       "Output logs in JSON",
		Destination: &config.App.Flags.JSON,
	},
	&cli.BoolFlag{
		Name:        "debug",
		Destination: &config.App.Flags.Debug,
	},
}

var Commands []*cli.Command

func RegisterCommand(command cli.Command) {
	Commands = append(Commands, &command)
}

func CommandNotFound(c *cli.Context, command string) {
	logrus.Errorf("%s: '%s' is not a %s command. See '%s --help'.", c.App.Name, command, c.App.Name, c.App.Name)
	os.Exit(2)
}
