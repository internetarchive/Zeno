package frontier

import (
	"path"
	"sync"

	"github.com/beeker1121/goque"
	"github.com/paulbellamy/ratecounter"
	"github.com/philippgille/gokv/leveldb"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

type Frontier struct {
	JobPath string

	// PullChan and PushChan are respectively the channels used for workers
	// to get new URLs to archive, and the channel to push the discovered URLs
	// to the frontier
	PullChan chan *Item
	PushChan chan *Item

	// Queue is a local queue storing all the URLs to crawl
	// it's a prefixed queue, basically one sub-queue per host
	QueueCount *ratecounter.Counter
	Queue      *goque.PrefixQueue

	// HostPool is an struct that contains a map and a Mutex.
	// the map contains all the different hosts that Zeno crawled,
	// with a counter for each, going through that map gives us
	// the prefix to query from the queue
	HostPool *HostPool

	UseSeencheck bool
	Seencheck    *Seencheck
}

// Init ininitialize the components of a frontier
func (f *Frontier) Init(jobPath string, logger *logrus.Logger, useSeencheck bool) (err error) {
	f.JobPath = jobPath

	log = logger

	// Initialize host pool
	f.HostPool = new(HostPool)
	f.HostPool.Mutex = new(sync.Mutex)
	f.HostPool.Hosts = make(map[string]*ratecounter.Counter, 0)

	// Initialize the frontier channels
	f.PullChan = make(chan *Item)
	f.PushChan = make(chan *Item)

	// Initialize the queue
	f.QueueCount = new(ratecounter.Counter)
	f.Queue, err = newPersistentQueue(jobPath)
	if err != nil {
		return err
	}
	logrus.Info("Persistent queue initialized")

	// Initialize the seencheck
	f.UseSeencheck = useSeencheck
	if f.UseSeencheck {
		f.Seencheck = new(Seencheck)
		f.Seencheck.SeenCount = new(ratecounter.Counter)
		f.Seencheck.SeenDB, err = leveldb.NewStore(leveldb.Options{Path: path.Join(jobPath, "seencheck")})
		if err != nil {
			return err
		}
		logrus.Info("Seencheck initialized")
	}

	return nil
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
