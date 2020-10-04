package crawl

import (
	"sync"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/warc"
	"github.com/gojektech/heimdall/v6/httpclient"
	"github.com/paulbellamy/ratecounter"
	"github.com/remeh/sizedwaitgroup"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"mvdan.cc/xurls/v2"
)

// Crawl define the parameters of a crawl process
type Crawl struct {
	SeedList []frontier.Item
	Mutex    *sync.Mutex

	// Frontier
	Frontier *frontier.Frontier

	// Crawl settings
	WorkerPool sizedwaitgroup.SizedWaitGroup
	Client     *httpclient.Client
	Log        *log.Entry
	Proxy      string
	UserAgent  string
	JobPath    string
	MaxHops    uint8
	Headless   bool
	Seencheck  bool
	Workers    int
	Finished   bool

	// Real time statistics
	URLsPerSecond *ratecounter.RateCounter
	ActiveWorkers *ratecounter.Counter
	Crawled       *ratecounter.Counter

	// WARC settings
	WARC             bool
	WARCPrefix       string
	WARCOperator     string
	WARCWriter       chan *warc.RecordBatch
	WARCWriterFinish chan bool

	// Kafka settings
	UseKafka           bool
	KafkaBrokers       []string
	KafkaConsumerGroup string
	KafkaFeedTopic     string
}

// Create initialize a Crawl structure and return it
func Create() (crawl *Crawl, err error) {
	crawl = new(Crawl)

	crawl.Crawled = new(ratecounter.Counter)
	crawl.ActiveWorkers = new(ratecounter.Counter)
	crawl.Frontier = new(frontier.Frontier)
	crawl.URLsPerSecond = ratecounter.NewRateCounter(1 * time.Second)

	return crawl, nil
}

// Finish handle the closing of the different crawl components
func (c *Crawl) Finish() {
	c.Finished = true
	c.WorkerPool.Wait()

	if c.WARC {
		close(c.WARCWriter)
		<-c.WARCWriterFinish
		logrus.Warning("WARC writer closed")
	}

	c.Frontier.Queue.Close()

	if c.Seencheck {
		c.Frontier.Seencheck.SeenDB.Close()
		logrus.Warning("Seencheck database closed")
	}

	logrus.Warning("Frontier queue closed")
}

// Start fire up the crawling process
func (c *Crawl) Start() (err error) {
	regexOutlinks = xurls.Relaxed()

	// Start the background process that will handle os signals
	// to exit Zeno, like CTRL+c
	c.setupCloseHandler()

	// Initialize the frontier
	c.Frontier.Init(c.JobPath, c.Seencheck)
	c.Frontier.Start()

	// If Kafka parameters are specified, then we start the background
	// process responsible for pulling seeds from Kafka
	if c.UseKafka {
		go c.KafkaConnector()
	} else {
		// Push the seed list to the queue
		for _, item := range c.SeedList {
			item := item
			c.Frontier.PushChan <- &item
		}
	}

	// Start archiving the URLs!
	for item := range c.Frontier.PullChan {
		if c.Finished {
			for {
				time.Sleep(1 * time.Minute)
			}
		}

		item := item

		c.WorkerPool.Add()
		go func(wg *sizedwaitgroup.SizedWaitGroup) {
			c.ActiveWorkers.Incr(1)
			c.Worker(item)
			wg.Done()
			c.ActiveWorkers.Incr(-1)
		}(&c.WorkerPool)
	}

	if c.Finished == false {
		c.Finish()
	}

	return nil
}
