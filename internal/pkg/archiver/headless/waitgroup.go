package headless

import (
	"maps"
	"sync"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/log"
)

// WaitGroup is a custom wait group that tracks inflight requests
// and provides methods to add, done, and retrieve inflight requests.
type WaitGroup struct {
	sync.WaitGroup
	sync.Mutex
	inflightReqs map[string]time.Time
}

func NewWaitGroup() *WaitGroup {
	return &WaitGroup{
		inflightReqs: make(map[string]time.Time),
	}
}

func (wg *WaitGroup) Add(delta int, url string) {
	wg.Mutex.Lock()
	defer wg.Mutex.Unlock()

	wg.WaitGroup.Add(delta)
	wg.inflightReqs[url] = time.Now()
}

func (wg *WaitGroup) Done(url string) {
	wg.Mutex.Lock()
	defer wg.Mutex.Unlock()

	wg.WaitGroup.Done()

	delete(wg.inflightReqs, url)
}

func (wg *WaitGroup) GetInflightReqs() map[string]time.Time {
	wg.Mutex.Lock()
	defer wg.Mutex.Unlock()

	// Return a copy of the map to
	// avoid concurrent map read and write errors
	reqsCopy := make(map[string]time.Time, len(wg.inflightReqs))
	maps.Copy(reqsCopy, wg.inflightReqs)
	return reqsCopy
}

func (wg *WaitGroup) ShowInflightReqs(logger *log.FieldedLogger, interval time.Duration) {
	for {
		time.Sleep(interval)

		if len(wg.inflightReqs) == 0 {
			return
		}

		tasks := wg.GetInflightReqs()
		logger.Debug("inflight requests", "count", len(tasks))
		for url, start := range tasks {
			logger.Debug("inflight request", "url", url, "elapsed", time.Since(start).String())
		}
	}
}

func (wg *WaitGroup) Wait(logger *log.FieldedLogger, interval time.Duration) {
	go wg.ShowInflightReqs(logger, interval)
	wg.WaitGroup.Wait()
}
