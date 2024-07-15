package crawl

import (
	"sync"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/frontier"
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
	currentItem  *frontier.Item
	previousItem *frontier.Item
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
// and eventually push newly discovered URLs back in the frontier.
func (w *Worker) Run() {
	// Start archiving the URLs!
	for {
		select {
		case <-w.doneSignal:
			w.Lock()
			w.state.currentItem = nil
			w.state.status = completed
			w.crawlParameters.Log.Info("Worker stopped", "worker", w.id)
			return
		case item := <-w.crawlParameters.Frontier.PullChan:
			// Can it happen? I don't think so but let's be safe
			if item == nil {
				continue
			}
			w.Lock()

			// If the crawl is paused, we wait until it's resumed
			for w.crawlParameters.Paused.Get() || w.crawlParameters.Frontier.Paused.Get() {
				time.Sleep(time.Second)
			}

			// If the host of the item is in the host exclusion list, we skip it
			if utils.StringInSlice(item.Host, w.crawlParameters.ExcludedHosts) || !w.crawlParameters.checkIncludedHosts(item.Host) {
				if w.crawlParameters.UseHQ {
					// If we are using the HQ, we want to mark the item as done
					w.crawlParameters.HQFinishedChannel <- item
				}
				w.Unlock()
				continue
			}

			// Launches the capture of the given item
			w.unsafeCapture(item)
			w.Unlock()
		}
	}
}

// unsafeCapture is named like so because it should only be called when the worker is locked
func (w *Worker) unsafeCapture(item *frontier.Item) {
	if item == nil {
		return
	}

	// Signals that the worker is processing an item
	w.crawlParameters.ActiveWorkers.Incr(1)
	w.state.currentItem = item
	w.state.status = processing

	// Capture the item
	err := w.crawlParameters.Capture(item)
	if err != nil {
		w.unsafePushLastError(err)
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

// unsafePushLastError is named like so because it should only be called when the worker is locked
func (w *Worker) unsafePushLastError(err error) {
	w.state.lastError = err
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
		doneSignal:      make(chan bool, 1),
		crawlParameters: crawlParameters,
	}
}
