package crawl

import (
	"sync"
	"time"

	"github.com/CorentinB/Zeno/pkg/queue"
	"github.com/beeker1121/goque"
	"github.com/gojektech/heimdall/v6/httpclient"
	"github.com/paulbellamy/ratecounter"
	"github.com/remeh/sizedwaitgroup"
	log "github.com/sirupsen/logrus"
)

// Crawl define the parameters of a crawl process
type Crawl struct {
	SeedList []queue.Item
	Mutex    *sync.Mutex

	// Queue
	Queue    *goque.PrefixQueue
	HostPool *HostPool

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
	crawl.HostPool = new(HostPool)
	crawl.HostPool.Mutex = new(sync.Mutex)
	crawl.HostPool.Hosts = make([]Host, 0)
	crawl.URLsPerSecond = ratecounter.NewRateCounter(1 * time.Second)

	// Initialize HTTP client
	timeout := 2000 * time.Millisecond
	crawl.Client = httpclient.NewClient(httpclient.WithHTTPTimeout(timeout))

	return crawl
}

// Start fire up the crawling process
func (c *Crawl) Start() (err error) {
	var wg = sizedwaitgroup.New(c.Workers)

	// Initialize the frontier channels
	pullChan := make(chan *queue.Item)
	pushChan := make(chan *queue.Item)

	// Initialize queue
	c.Queue, err = queue.NewQueue()
	if err != nil {
		return err
	}
	defer c.Queue.Close()

	// Start the workers
	for i := 0; i < c.Workers; i++ {
		wg.Add()
		go c.Worker(pullChan, pushChan, &wg)
	}

	c.Manager(pushChan, pullChan)

	// Push the seed list to the queue
	for _, item := range c.SeedList {
		item := item
		pushChan <- &item
	}

	// Wait for workers to finish and drop the local queue
	wg.Wait()
	err = c.Queue.Drop()
	if err != nil {
		return nil
	}

	return nil
}
