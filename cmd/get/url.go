package get

import (
	"net/url"

	"github.com/internetarchive/Zeno/cmd"
	"github.com/internetarchive/Zeno/config"
	"github.com/internetarchive/Zeno/internal/pkg/queue"
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
	err := initLogging()
	if err != nil {
		logrus.Error("Unable to parse arguments")
		return err
	}

	// Init crawl using the flags provided
	crawl := cmd.InitCrawlWithCMD(config.App.Flags)

	// Initialize initial seed list
	input, err := url.Parse(c.Args().Get(0))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"input": c.Args().Get(0),
			"err":   err.Error(),
		}).Error("This is not a valid input")
		return err
	}

	crawl.SeedList = append(crawl.SeedList, *queue.NewItem(input, nil, "seed", 0, "", false))

	// Start crawl
	err = crawl.Start()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"crawl": crawl,
			"err":   err.Error(),
		}).Error("Crawl exited due to error")
		return err
	}

	logrus.Info("Crawl finished")
	return err
}
