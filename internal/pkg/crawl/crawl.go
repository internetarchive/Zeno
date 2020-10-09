package crawl

import (
	"sync"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/CorentinB/warc"
	"github.com/gojektech/heimdall/v6/httpclient"
	"github.com/paulbellamy/ratecounter"
	"github.com/remeh/sizedwaitgroup"
	"github.com/sirupsen/logrus"
	"mvdan.cc/xurls/v2"
)

var log *logrus.Logger

// Crawl define the parameters of a crawl process
type Crawl struct {
	*sync.Mutex
	StartTime time.Time
	SeedList  []frontier.Item
	Finished  *utils.TAtomBool

	// Frontier
	Frontier *frontier.Frontier

	// Crawl settings
	WorkerPool   sizedwaitgroup.SizedWaitGroup
	Client       *httpclient.Client
	Logger       logrus.Logger
	Proxy        string
	UserAgent    string
	Job          string
	JobPath      string
	MaxHops      uint8
	MaxRedirect  int
	DomainsCrawl bool
	Headless     bool
	Seencheck    bool
	Workers      int

	// API settings
	API bool

	// Real time statistics
	URLsPerSecond *ratecounter.RateCounter
	ActiveWorkers *ratecounter.Counter
	Crawled       *ratecounter.Counter

	// WARC settings
	WARC             bool
	WARCRetry        int
	WARCPrefix       string
	WARCOperator     string
	WARCWriter       chan *warc.RecordBatch
	WARCWriterFinish chan bool

	// Kafka settings
	UseKafka             bool
	KafkaBrokers         []string
	KafkaConsumerGroup   string
	KafkaFeedTopic       string
	KafkaOutlinksTopic   string
	KafkaProducerChannel chan *frontier.Item
}

// Start fire up the crawling process
func (c *Crawl) Start() (err error) {
	c.StartTime = time.Now()
	c.Finished = new(utils.TAtomBool)
	regexOutlinks = xurls.Relaxed()

	// Setup logging
	log = utils.SetupLogging(c.JobPath)

	// Initialize HTTP client
	c.initHTTPClient()

	// Start the background process that will handle os signals
	// to exit Zeno, like CTRL+C
	go c.setupCloseHandler()

	// Initialize the frontier
	c.Frontier.Init(c.JobPath, log, c.Workers, c.Seencheck)
	c.Frontier.Load()
	c.Frontier.Start()

	// Function responsible for writing to disk the frontier's hosts pool
	// and other stats needed to resume the crawl. The process happen every minute.
	// The actual queue used during the crawl and seencheck aren't included in this,
	// because they are written to disk in real-time.
	go c.writeFrontierToDisk()

	// Start the background process that will catch when there
	// is nothing more to crawl
	if !c.UseKafka {
		go c.catchFinish()
	}

	if c.API {
		go c.startAPI()
	}

	// If Kafka parameters are specified, then we start the background
	// processes responsible for pulling and pushing seeds from and to Kafka
	if c.UseKafka {
		c.KafkaProducerChannel = make(chan *frontier.Item, c.Workers)
		go c.kafkaConsumer()
		if len(c.KafkaOutlinksTopic) > 0 {
			go c.kafkaProducer()
		}
	} else {
		// Push the seed list to the queue
		logrus.Info("Pushing seeds in the local queue..")
		for _, item := range c.SeedList {
			item := item
			c.Frontier.PushChan <- &item
		}
		c.SeedList = nil
		logrus.Info("All seeds are now in queue, crawling will start")
	}

	// Start the process responsible for printing live stats on the standard output
	go c.printLiveStats()

	// Start archiving the URLs!
	for item := range c.Frontier.PullChan {
		if c.Finished.Get() {
			for {
				time.Sleep(1 * time.Minute)
			}
		}

		item := item

		c.WorkerPool.Add()
		go func(wg *sizedwaitgroup.SizedWaitGroup) {
			c.ActiveWorkers.Incr(1)
			c.Capture(item)
			wg.Done()
			c.ActiveWorkers.Incr(-1)
		}(&c.WorkerPool)
	}

	if c.Finished.Get() == false {
		c.finish()
	}

	return nil
}
