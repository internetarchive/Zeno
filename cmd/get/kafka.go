package get

import (
	"path"

	"github.com/CorentinB/Zeno/config"
	"github.com/CorentinB/Zeno/internal/pkg/crawl"
	"github.com/google/uuid"
	"github.com/remeh/sizedwaitgroup"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func NewGetKafkaCmd() *cli.Command {
	return &cli.Command{
		Name:      "kafka",
		Usage:     "Start crawling with the Kafka connector",
		Action:    CmdGetKafka,
		Flags:     []cli.Flag{},
		UsageText: "<FILE> [ARGUMENTS]",
	}
}

func CmdGetKafka(c *cli.Context) error {
	err := initLogging(c)
	if err != nil {
		log.Error("Unable to parse arguments")
		return err
	}

	// Initialize Crawl
	crawl, err := crawl.Create()
	if err != nil {
		log.Error("Unable to initialize crawl job")
		return err
	}

	// If the job name isn't specified, we generate a random name
	if len(config.App.Flags.Job) == 0 {
		UUID, err := uuid.NewUUID()
		if err != nil {
			return err
		}
		config.App.Flags.Job = UUID.String()
	}

	crawl.JobPath = path.Join("jobs", config.App.Flags.Job)
	crawl.UserAgent = config.App.Flags.UserAgent
	crawl.Headless = config.App.Flags.Headless
	crawl.WARC = config.App.Flags.WARC
	crawl.Workers = config.App.Flags.Workers
	crawl.WorkerPool = sizedwaitgroup.New(crawl.Workers)
	crawl.Seencheck = config.App.Flags.Seencheck
	crawl.MaxHops = uint8(config.App.Flags.MaxHops)
	crawl.Log = log.WithFields(log.Fields{
		"crawl": crawl,
	})

	// Kafka-specific settings
	crawl.UseKafka = true
	crawl.KafkaConsumerGroup = config.App.Flags.KafkaConsumerGroup
	crawl.KafkaFeedTopic = config.App.Flags.KafkaFeedTopic
	crawl.KafkaBrokers = config.App.Flags.KafkaBrokers.Value()

	// Initialize client
	crawl.InitHTTPClient()

	// Start crawl
	err = crawl.Start()
	if err != nil {
		log.WithFields(log.Fields{
			"crawl": crawl,
			"error": err,
		}).Error("Crawl exited due to error")
		return err
	}

	return nil
}
