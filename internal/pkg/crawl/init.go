package crawl

import (
	"sync"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/gojektech/heimdall/v6/httpclient"
	"github.com/paulbellamy/ratecounter"
	"github.com/remeh/sizedwaitgroup"
	log "github.com/sirupsen/logrus"
)

// Crawl define the parameters of a crawl process
type Crawl struct {
	SeedList []frontier.Item
	Mutex    *sync.Mutex

	// Frontier
	Frontier *frontier.Frontier

	// Crawl settings
	Client   *httpclient.Client
	Log      *log.Entry
	MaxHops  uint8
	Headless bool
	Workers  int

	// Real time statistics
	URLsPerSecond *ratecounter.RateCounter
	ActiveWorkers *ratecounter.Counter
}

// Create initialize a Crawl structure and return it
func Create() *Crawl {
	crawl := new(Crawl)
	crawl.ActiveWorkers = new(ratecounter.Counter)
	crawl.Frontier = new(frontier.Frontier)
	crawl.URLsPerSecond = ratecounter.NewRateCounter(1 * time.Second)

	// Initialize HTTP client
	timeout := 2000 * time.Millisecond
	crawl.Client = httpclient.NewClient(httpclient.WithHTTPTimeout(timeout))

	return crawl
}

// Start fire up the crawling process
func (c *Crawl) Start() (err error) {
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
