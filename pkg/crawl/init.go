package crawl

import (
	"github.com/CorentinB/Zeno/pkg/queue"
	"github.com/beeker1121/goque"
	log "github.com/sirupsen/logrus"
	"github.com/remeh/sizedwaitgroup"
	"net/url"
)

// Crawl define the parameters of a crawl process
type Crawl struct {
	Origin         *url.URL
	Log            *log.Entry
	Queue          *goque.PriorityQueue
	ReceiverChan   *chan *queue.Item
	DispatcherChan *chan *queue.Item
	MaxHops        uint8
	Workers int
}

// Create initialize a Crawl structure and return it
func Create() *Crawl {
	return new(Crawl)
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

	// Push the original seed to the queue
	originalItem := queue.NewItem(c.Origin, nil, 0)
	writerChan <- originalItem

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
