package crawl

import (
	"sync"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/gojektech/heimdall"
	"github.com/gojektech/heimdall/v6/httpclient"
	"github.com/paulbellamy/ratecounter"
	"github.com/remeh/sizedwaitgroup"
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
	Client    *httpclient.Client
	Log       *log.Entry
	MaxHops   uint8
	Headless  bool
	Seencheck bool
	Workers   int

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

	// Initialize HTTP client
	var maximumJitterInterval time.Duration = 2 * time.Millisecond // Max jitter interval
	var initalTimeout time.Duration = 2 * time.Millisecond         // Inital timeout
	var maxTimeout time.Duration = 9 * time.Millisecond            // Max time out
	var timeout time.Duration = 1000 * time.Millisecond
	var exponentFactor float64 = 2 // Multiplier

	backoff := heimdall.NewExponentialBackoff(initalTimeout, maxTimeout, exponentFactor, maximumJitterInterval)
	retrier := heimdall.NewRetrier(backoff)

	// Create a new client, sets the retry mechanism, and the number of retries
	crawl.Client = httpclient.NewClient(
		httpclient.WithHTTPTimeout(timeout),
		httpclient.WithRetrier(retrier),
		httpclient.WithRetryCount(4),
	)

	return crawl, nil
}

// Start fire up the crawling process
func (c *Crawl) Start() (err error) {
	regexOutlinks = xurls.Relaxed()
	var wg = sizedwaitgroup.New(c.Workers)

	// Initialize the frontier
	c.Frontier.Init()
	c.Frontier.Start()
	defer c.Frontier.Stop()

	// Start the workers
	for i := 0; i < c.Workers; i++ {
		wg.Add()
		go c.Worker(&wg)
	}

	// Push the seed list to the queue
	for _, item := range c.SeedList {
		item := item
		c.Frontier.PushChan <- &item
	}

	// Wait for workers to finish
	wg.Wait()

	return nil
}
