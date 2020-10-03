package crawl

import (
	"os"
	"sync"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/warc"
	"github.com/gojektech/heimdall/v6/httpclient"
	"github.com/paulbellamy/ratecounter"
	"github.com/remeh/sizedwaitgroup"
	log "github.com/sirupsen/logrus"
	"mvdan.cc/xurls/v2"
)

var (
	WARCWriter       chan *warc.RecordBatch
	WARCWriterFinish chan bool
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
	JobPath    string
	MaxHops    uint8
	Headless   bool
	Seencheck  bool
	Workers    int
	WARC       bool

	// Real time statistics
	URLsPerSecond *ratecounter.RateCounter
	ActiveWorkers *ratecounter.Counter
	Crawled       *ratecounter.Counter
}

// Create initialize a Crawl structure and return it
func Create() (crawl *Crawl, err error) {
	crawl = new(Crawl)
	crawl.Crawled = new(ratecounter.Counter)
	crawl.ActiveWorkers = new(ratecounter.Counter)
	crawl.Frontier = new(frontier.Frontier)
	crawl.URLsPerSecond = ratecounter.NewRateCounter(1 * time.Second)

	crawl.WorkerPool = sizedwaitgroup.New(crawl.Workers)

	return crawl, nil
}

// Finish handle the closing of the different crawl components
func (c *Crawl) Finish() {
	c.Frontier.Seencheck.SeenDB.Close()

	os.Exit(0)
}

// Start fire up the crawling process
func (c *Crawl) Start() (err error) {
	regexOutlinks = xurls.Relaxed()
	var wg = sizedwaitgroup.New(c.Workers)

	// Initialize the frontier
	c.Frontier.Init(c.JobPath, c.Seencheck)
	c.Frontier.Start()

	// Push the seed list to the queue
	for _, item := range c.SeedList {
		item := item
		c.Frontier.PushChan <- &item
	}

	// Start archiving the URLs!
	for item := range c.Frontier.PullChan {
		wg.Add()
		item := item
		go func() {
			c.Worker(item)
			wg.Done()
		}()
	}

	// Wait for workers to finish
	wg.Wait()

	// Close WARC writer
	if c.WARC {
		WARCWriterFinish <- true
	}

	c.Frontier.Stop()

	return nil
}
