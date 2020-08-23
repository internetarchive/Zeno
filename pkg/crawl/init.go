package crawl

import (
	"github.com/CorentinB/Zeno/pkg/queue"
	"github.com/beeker1121/goque"
	"github.com/paulbellamy/ratecounter"
	"github.com/remeh/sizedwaitgroup"
	log "github.com/sirupsen/logrus"
	"time"
)

// Crawl define the parameters of a crawl process
type Crawl struct {
	SeedList []queue.Item
	Log            *log.Entry
	Queue          *goque.PriorityQueue
	ReceiverChan   *chan *queue.Item
	DispatcherChan *chan *queue.Item
	MaxHops        uint8
	Workers int
	URLsPerSecond *ratecounter.RateCounter
	Headless bool
}

// Create initialize a Crawl structure and return it
func Create() *Crawl {
	crawl := new(Crawl)
	crawl.URLsPerSecond = ratecounter.NewRateCounter(1 * time.Second)

	return crawl
}

// Start fire up the crawling process
func (c *Crawl) Start() (err error) {
	var wg = sizedwaitgroup.New(c.Workers)

	// Create the crawling queue
	c.Queue, err = queue.NewQueue()
	if err != nil {
		return err
	}
	defer c.Queue.Close()

	// Start the queue writer
	writerChan := queue.NewWriter()
	go queue.StartWriter(writerChan, c.Queue)

	// Push the seed list to the queue
	for _, item := range c.SeedList {
		writerChan <- &item
	}

	// Start the workers
	for i := 0; i < c.Workers; i++ {
		wg.Add()
		go c.Worker(writerChan, &wg)
	}

	// Wait for workers to finish and drop the local queue
	wg.Wait()
	err = c.Queue.Drop()
	if err != nil {
		return nil
	}

	return nil
}
