package frontier

import (
	"sync"

	"github.com/beeker1121/goque"
)

type Frontier struct {
	// PullChan and PushChan are respectively the channels used for workers
	// to get new URLs to archive, and the channel to push the discovered URLs
	// to the frontier
	PullChan chan *Item
	PushChan chan *Item

	// Queue is a local queue storing all the URLs to crawl
	// it's a prefixed queue, basically one sub-queue per host
	Queue *goque.PrefixQueue

	// HostPool is an array that contains all the different hosts
	// that Zeno crawled, with a counter for each, going through
	// that pull gives us the prefix to query from the queue
	HostPool *HostPool
}

// Init ininitialize the components of a frontier
func (f *Frontier) Init() (err error) {
	// Initialize host pool
	f.HostPool = new(HostPool)
	f.HostPool.Mutex = new(sync.Mutex)
	f.HostPool.Hosts = make([]Host, 0)

	// Initialize the frontier channels
	f.PullChan = make(chan *Item)
	f.PushChan = make(chan *Item)

	// Initialize the queue
	f.Queue, err = newPersistentQueue()
	if err != nil {
		return err
	}

	return nil
}

// Stop close the frontier's components properly
func (f *Frontier) Stop() {
	defer f.Queue.Close()

	_ = f.Queue.Drop()
}

// Start fire up the background processes that handle the frontier
func (f *Frontier) Start() {
	// Function responsible for writing the items push on PushChan to the
	// local queue, items received on this channels are typically initial seeds
	// or outlinks discovered on web pages
	go f.writeItemsToQueue()

	// Function responsible for reading the items from the queue and dispatching
	// them to the workers listening on PullChan
	go f.readItemsFromQueue()
}
