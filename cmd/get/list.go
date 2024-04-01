package get

import (
	"github.com/internetarchive/Zeno/cmd"
	"github.com/internetarchive/Zeno/config"
	"github.com/internetarchive/Zeno/internal/pkg/frontier"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func newGetListCmd() *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "Start crawling with a seed list.",
		Action:    cmdGetList,
		Flags:     []cli.Flag{},
		UsageText: "<FILE> [ARGUMENTS]",
	}
}

func cmdGetList(c *cli.Context) error {
	err := initLogging()
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
			"err":   err.Error(),
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
			"err":   err.Error(),
		}).Error("Crawl exited due to error")
		return err
	}

	return nil
}
