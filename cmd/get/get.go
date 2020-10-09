package get

import (
	"github.com/CorentinB/Zeno/cmd"
	"github.com/CorentinB/Zeno/config"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func initLogging(c *cli.Context) (err error) {
	// Log as JSON instead of the default ASCII formatter.
	if config.App.Flags.JSON {
		log.SetFormatter(&log.JSONFormatter{})
	}

	// Turn on debug mode
	if config.App.Flags.Debug {
		log.SetLevel(log.DebugLevel)
	}

	return nil
}

func init() {
	cmd.RegisterCommand(
		cli.Command{
			Name:  "get",
			Usage: "All commands that get URLs",
			Subcommands: []*cli.Command{
				newGetURLCmd(),
				newGetListCmd(),
				newGetKafkaCmd(),
			},
		})
}
