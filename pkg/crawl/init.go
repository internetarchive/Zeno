package crawl

import (
	"net/url"
	"time"

	"github.com/CorentinB/Zeno/pkg/queue"
	"github.com/beeker1121/goque"
	log "github.com/sirupsen/logrus"
)

// Crawl define the parameters of a crawl process
type Crawl struct {
	Origin         *url.URL
	Log            *log.Entry
	Queue          *goque.PriorityQueue
	ReceiverChan   *chan *queue.Item
	DispatcherChan *chan *queue.Item
	MaxHops        int
}

// Create initialize a Crawl structure and return it
func Create() *Crawl {
	return new(Crawl)
}

// Start fire up the crawling process
func (c *Crawl) Start() (err error) {
	// Create the crawling queue
	c.Queue, err = queue.NewQueue()
	if err != nil {
		return err
	}
	defer c.Queue.Close()

	// Start the queue writer
	writerChan := queue.NewWriter()
	go queue.StartWriter(writerChan, c.Queue)

	// TODO Start the workers
	go Worker(writerChan, c.Queue)

	// Push the original seed to the queue
	originalItem := queue.NewItem(c.Origin, nil, 0)
	writerChan <- originalItem

	// TODO Wait for the jobs to finish
	time.Sleep(10 * time.Second)

	return nil
}
