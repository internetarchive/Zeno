package get

import (
	"net/url"

	"github.com/CorentinB/Zeno/cmd"
	"github.com/CorentinB/Zeno/config"
	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func newGetURLCmd() *cli.Command {
	return &cli.Command{
		Name:      "url",
		Usage:     "Start crawling with a single URL.",
		Action:    cmdGetURL,
		Flags:     []cli.Flag{},
		UsageText: "<URL> [ARGUMENTS]",
	}
}

func cmdGetURL(c *cli.Context) error {
	err := initLogging(c)
	if err != nil {
		logrus.Error("Unable to parse arguments")
		return err
	}

	// Init crawl using the flags provided
	crawl := cmd.InitCrawlWithCMD(config.App.Flags)

	// Initialize initial seed list
	input, err := url.Parse(c.Args().Get(0))
	err = utils.ValidateURL(input)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"input": c.Args().Get(0),
			"error": err.Error(),
		}).Error("This is not a valid input")
		return err
	}
	crawl.SeedList = append(crawl.SeedList, *frontier.NewItem(input, nil, "seed", 0, ""))

	// Start crawl
	err = crawl.Start()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"crawl": crawl,
			"error": err,
		}).Error("Crawl exited due to error")
		return err
	}

	logrus.Info("Crawl finished")
	return err
}
