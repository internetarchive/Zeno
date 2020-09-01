package get

import (
	"net/url"

	"github.com/CorentinB/Zeno/config"
	"github.com/CorentinB/Zeno/pkg/crawl"
	"github.com/CorentinB/Zeno/pkg/queue"
	"github.com/CorentinB/Zeno/pkg/utils"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func NewGetURLCmd() cli.Command {
	return cli.Command{
		Name:      "url",
		Usage:     "Start crawling with a single URL",
		Action:    CmdGetURL,
		Flags:     []cli.Flag{},
		UsageText: "<URL> [ARGUMENTS]",
	}
}

func CmdGetURL(c *cli.Context) {
	err := initLogging(c)
	if err != nil {
		log.Fatal("Unable to parse arguments")
	}

	// Initialize Crawl
	crawl := crawl.Create()
	crawl.Headless = config.App.Flags.Headless
	crawl.Workers = config.App.Flags.Workers
	crawl.MaxHops = uint8(config.App.Flags.MaxHops)
	crawl.Log = log.WithFields(log.Fields{
		"crawl": crawl,
	})

	// Initialize initial seed list
	input, err := url.Parse(c.Args().Get(0))
	err = utils.ValidateURL(input)
	if err != nil {
		log.WithFields(log.Fields{
			"input": c.Args().Get(0),
			"error": err.Error(),
		}).Fatal("This is not a valid input")
	}
	crawl.SeedList = append(crawl.SeedList, *queue.NewItem(input, nil, 0))

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
