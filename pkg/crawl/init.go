package crawl

import (
	"container/list"
	"sync"
	"time"

	"github.com/CorentinB/Zeno/pkg/queue"
	"github.com/gojektech/heimdall/v6/httpclient"
	"github.com/paulbellamy/ratecounter"
	"github.com/remeh/sizedwaitgroup"
	log "github.com/sirupsen/logrus"
)

// Crawl define the parameters of a crawl process
type Crawl struct {
	Client        *httpclient.Client
	SeedList      []queue.Item
	Log           *log.Entry
	Queue         *list.List
	MaxHops       uint8
	Workers       int
	URLsPerSecond *ratecounter.RateCounter
	ActiveWorkers int
	Headless      bool
}

// Create initialize a Crawl structure and return it
func Create() *Crawl {
	crawl := new(Crawl)
	crawl.URLsPerSecond = ratecounter.NewRateCounter(1 * time.Second)

	// Initialize queue
	crawl.Queue = list.New()

	// Initialize HTTP client
	timeout := 2000 * time.Millisecond
	crawl.Client = httpclient.NewClient(httpclient.WithHTTPTimeout(timeout))

	return crawl
}

// Start fire up the crawling process
func (c *Crawl) Start() (err error) {
	var wg = sizedwaitgroup.New(c.Workers)
	var m sync.Mutex

	// Initialize the frontier
	pullChan := make(chan *queue.Item)
	pushChan := make(chan *queue.Item)

	// Start the frontiers
	for i := 0; i < c.Workers; i++ {
		wg.Add()
		go c.Worker(pullChan, pushChan, &wg, &m)
	}

	c.Manager(pushChan, pullChan)

	// Push the seed list to the queue
	for _, item := range c.SeedList {
		item := item
		pushChan <- &item
	}

	// Wait for workers to finish and drop the local queue
	wg.Wait()

	return nil
}
