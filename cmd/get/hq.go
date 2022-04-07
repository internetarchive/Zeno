package get

import (
	"github.com/CorentinB/Zeno/cmd"
	"github.com/CorentinB/Zeno/config"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func newGetHQCmd() *cli.Command {
	return &cli.Command{
		Name:      "hq",
		Usage:     "Start crawling with the crawl HQ connector",
		Action:    cmdGetHQ,
		Flags:     []cli.Flag{},
		UsageText: "<FILE> [ARGUMENTS]",
	}
}

func cmdGetHQ(c *cli.Context) error {
	err := initLogging(c)
	if err != nil {
		log.Error("Unable to parse arguments")
		return err
	}

	// init crawl using the flags provided
	crawl := cmd.InitCrawlWithCMD(config.App.Flags)

	// start crawl
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
