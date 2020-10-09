package get

import (
	"github.com/CorentinB/Zeno/cmd"
	"github.com/CorentinB/Zeno/config"
	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func newGetListCmd() *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "Start crawling with a seed list",
		Action:    cmdGetList,
		Flags:     []cli.Flag{},
		UsageText: "<FILE> [ARGUMENTS]",
	}
}

func cmdGetList(c *cli.Context) error {
	err := initLogging(c)
	if err != nil {
		log.Error("Unable to parse arguments")
		return err
	}

	// Init crawl using the flags provided
	crawl := cmd.InitCrawlWithCMD(config.App.Flags)

	// Initialize initial seed list
	crawl.SeedList, err = frontier.IsSeedList(c.Args().Get(0))
	if err != nil || len(crawl.SeedList) <= 0 {
		logrus.WithFields(logrus.Fields{
			"input": c.Args().Get(0),
			"error": err.Error(),
		}).Error("This is not a valid input")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"input":      c.Args().Get(0),
		"seedsCount": len(crawl.SeedList),
	}).Print("Seed list loaded")

	// Start crawl
	err = crawl.Start()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"crawl": crawl,
			"error": err,
		}).Error("Crawl exited due to error")
		return err
	}

	return nil
}
