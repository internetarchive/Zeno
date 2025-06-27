package headless

import (
	"maps"
	"sync"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/log"
)

// WaitGroup is a custom wait group that tracks ongoing requests
// and provides methods to add, done, and retrieve ongoing requests.
type WaitGroup struct {
	sync.WaitGroup
	sync.Mutex
	ongoingReqs map[string]time.Time
}

func NewWaitGroup() *WaitGroup {
	return &WaitGroup{
		ongoingReqs: make(map[string]time.Time),
	}
}

func (wg *WaitGroup) Add(delta int, url string) {
	wg.Mutex.Lock()
	defer wg.Mutex.Unlock()

	wg.WaitGroup.Add(delta)
	wg.ongoingReqs[url] = time.Now()
}

func (wg *WaitGroup) Done(url string) {
	wg.Mutex.Lock()
	defer wg.Mutex.Unlock()

	wg.WaitGroup.Done()

	delete(wg.ongoingReqs, url)
}

func (wg *WaitGroup) GetOngoingReqs() map[string]time.Time {
	wg.Mutex.Lock()
	defer wg.Mutex.Unlock()

	// Return a copy of the map to
	// avoid concurrent map read and write errors
	reqsCopy := make(map[string]time.Time, len(wg.ongoingReqs))
	maps.Copy(reqsCopy, wg.ongoingReqs)
	return reqsCopy
}

func (wg *WaitGroup) ShowOngoingReqs(logger *log.FieldedLogger, interval time.Duration) {
	for {
		time.Sleep(interval)

		if len(wg.ongoingReqs) == 0 {
			return
		}

		tasks := wg.GetOngoingReqs()
		logger.Debug("ongoing requests", "count", len(tasks))
		for url, start := range tasks {
			logger.Debug("ongoing request", "url", url, "elapsed", time.Since(start).String())
		}
	}
}

func (wg *WaitGroup) Wait(logger *log.FieldedLogger, interval time.Duration) {
	go wg.ShowOngoingReqs(logger, interval)
	wg.WaitGroup.Wait()
}
