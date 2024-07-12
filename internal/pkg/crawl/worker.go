package crawl

import (
	"sync"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/queue"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
)

const (
	// B represent a Byte
	B = 1
	// KB represent a Kilobyte
	KB = 1024 * B
	// MB represent a MegaByte
	MB = 1024 * KB
	// GB represent a GigaByte
	GB = 1024 * MB
)

type status int

const (
	idle status = iota
	processing
	completed
)

func (s status) String() string {
	statusStr := map[status]string{
		idle:       "idle",
		processing: "processing",
		completed:  "completed",
	}
	return statusStr[s]
}

type workerState struct {
	currentItem  *queue.Item
	previousItem *queue.Item
	status       status
	lastError    error
	lastSeen     time.Time
}

type Worker struct {
	sync.Mutex
	id              uint
	state           *workerState
	doneSignal      chan bool
	crawlParameters *Crawl
}

// Run is the key component of a crawl, it's a background processed dispatched
// when the crawl starts, it listens on a channel to get new URLs to archive,
// and eventually push newly discovered URLs back in the queue.
func (w *Worker) Run() {
	// Start archiving the URLs!
	for {
		item, err := w.crawlParameters.Queue.Dequeue()
		if err != nil {
			// Log the error too?
			w.PushLastError(err)
			continue
		}

		// Check if the crawl is paused or needs to be stopped
		select {
		case <-w.doneSignal:
			w.Lock()
			w.state.currentItem = nil
			w.state.status = completed
			return
		default:
			for w.crawlParameters.Paused.Get() {
				time.Sleep(time.Second)
			}
		}

		// If the host of the item is in the host exclusion list, we skip it
		if utils.StringInSlice(item.Host, w.crawlParameters.ExcludedHosts) || !w.crawlParameters.checkIncludedHosts(item.Host) {
			if w.crawlParameters.UseHQ {
				// If we are using the HQ, we want to mark the item as done
				w.crawlParameters.HQFinishedChannel <- item
			}

			continue
		}

		// Launches the capture of the given item
		w.Capture(item)
	}
}

func (w *Worker) Capture(item *queue.Item) {
	// Locks the worker
	w.Lock()
	defer w.Unlock()

	// Signals that the worker is processing an item
	w.crawlParameters.ActiveWorkers.Incr(1)
	w.state.currentItem = item
	w.state.status = processing

	// Capture the item
	err := w.crawlParameters.Capture(item)
	if err != nil {
		w.PushLastError(err)
	}

	// Signals that the worker has finished processing the item
	w.state.status = idle
	w.state.currentItem = nil
	w.state.previousItem = item
	w.crawlParameters.ActiveWorkers.Incr(-1)
	w.state.lastSeen = time.Now()
}

func (w *Worker) Stop() {
	w.doneSignal <- true
}

func (w *Worker) PushLastError(err error) {
	w.Lock()
	w.state.lastError = err
	w.Unlock()
}

func newWorker(crawlParameters *Crawl, id uint) *Worker {
	return &Worker{
		id: id,
		state: &workerState{
			status:       idle,
			previousItem: nil,
			currentItem:  nil,
			lastError:    nil,
		},
		doneSignal:      make(chan bool),
		crawlParameters: crawlParameters,
	}
}
