package get

import (
	"github.com/CorentinB/Zeno/config"
	"github.com/CorentinB/Zeno/internal/pkg/crawl"
	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func NewGetListCmd() cli.Command {
	return cli.Command{
		Name:      "list",
		Usage:     "Start crawling with a seed list",
		Action:    CmdGetList,
		Flags:     []cli.Flag{},
		UsageText: "<FILE> [ARGUMENTS]",
	}
}

func CmdGetList(c *cli.Context) {
	err := initLogging(c)
	if err != nil {
		log.Fatal("Unable to parse arguments")
	}

	// Initialize Crawl
	crawl, err := crawl.Create()
	if err != nil {
		log.Fatal("Unable to initialize crawl job")
	}
	crawl.Headless = config.App.Flags.Headless
	crawl.Workers = config.App.Flags.Workers
	crawl.MaxHops = uint8(config.App.Flags.MaxHops)
	crawl.Log = log.WithFields(log.Fields{
		"crawl": crawl,
	})

	// Initialize initial seed list
	crawl.SeedList, err = frontier.IsSeedList(c.Args().Get(0))
	if err != nil || len(crawl.SeedList) <= 0 {
		log.WithFields(log.Fields{
			"input": c.Args().Get(0),
			"error": err.Error(),
		}).Fatal("This is not a valid input")
	}

	log.WithFields(log.Fields{
		"input":      c.Args().Get(0),
		"seedsCount": len(crawl.SeedList),
	}).Print("Seed list loaded")

	// Start crawl
	err = crawl.Start()
	if err != nil {
		log.WithFields(log.Fields{
			"crawl": crawl,
			"error": err,
		}).Fatal("Crawl exited due to error")
	}

	crawl.Log.Info("Crawl finished")
}
