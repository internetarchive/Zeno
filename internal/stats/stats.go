package stats

import (
	"fmt"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/CorentinB/warc"
	"github.com/dustin/go-humanize"
	"github.com/gosuri/uilive"
	"github.com/gosuri/uitable"
)

type Runner struct {
	StopChan chan struct{}
	DoneChan chan struct{}
	data     *data
}

var initialized *atomic.Bool

// Init initializes the stats package
//
// Returns:
// - Runner: a struct that can be used to control the stats lifecycle
// - bool: true if the stats package was initialized, false otherwise meaning that stats was already initialized
func Init() (*Runner, bool) {
	if initialized.Load() {
		return nil, false
	}

	data := initStatsData()
	runner := &Runner{
		StopChan: make(chan struct{}),
		DoneChan: make(chan struct{}),
		data:     data,
	}

	initialized.Store(true)

	return runner, true
}

// Printer starts the stats printer.
// Preferably run this in a goroutine.
// Is controlled by the StopChan channel.
func (r *Runner) Printer() {
	var stats *uitable.Table
	var m runtime.MemStats

	writer := uilive.New()
	writer.Start()

	for {
		select {
		case <-r.StopChan:
			writer.Stop()
			r.DoneChan <- struct{}{}
			return
		default:
			runtime.ReadMemStats(&m)

			stats = uitable.New()
			stats.MaxColWidth = 80
			stats.Wrap = true

			crawledSeeds := r.GetCrawledSeeds()
			crawledAssets := r.GetCrawledAssets()

			queueStats := r.GetQueueStats()

			stats.AddRow("", "")
			stats.AddRow("  - Job:", r.GetJob())
			stats.AddRow("  - State:", r.GetCrawlState())
			stats.AddRow("  - Active workers:", strconv.Itoa(int(c.ActiveWorkers.Value()))+"/"+strconv.Itoa(c.Workers.wpLen()))
			stats.AddRow("  - URI/s:", c.URIsPerSecond.Rate())
			stats.AddRow("  - Items in queue:", queueStats.TotalElements)
			stats.AddRow("  - Hosts in queue:", queueStats.UniqueHosts)
			if c.UseHandover {
				stats.AddRow("  - Handover open:", c.Queue.HandoverOpen.Get())
				stats.AddRow("  - Handover Get() success:", queueStats.HandoverSuccessGetCount)
			}
			stats.AddRow("  - Queue empty bool state:", c.Queue.Empty.Get())
			stats.AddRow("  - Can Enqueue:", c.Queue.CanEnqueue())
			stats.AddRow("  - Can Dequeue:", c.Queue.CanDequeue())
			stats.AddRow("  - Crawled total:", crawledSeeds+crawledAssets)
			stats.AddRow("  - Crawled seeds:", crawledSeeds)
			stats.AddRow("  - Crawled assets:", crawledAssets)
			stats.AddRow("  - WARC writing queue:", c.Client.WaitGroup.Size())
			stats.AddRow("  - Data written:", humanize.Bytes(uint64(warc.DataTotal.Value())))

			if !c.DisableLocalDedupe {
				stats.AddRow("  - Deduped (local):", humanize.Bytes(uint64(warc.LocalDedupeTotal.Value())))
			}

			if c.CDXDedupeServer != "" {
				stats.AddRow("  - Deduped (via CDX):", humanize.Bytes(uint64(warc.RemoteDedupeTotal.Value())))
			}

			stats.AddRow("", "")
			stats.AddRow("  - Elapsed time:", time.Since(c.StartTime).String())
			stats.AddRow("  - Allocated (heap):", bToMb(m.Alloc))
			stats.AddRow("  - Goroutines:", runtime.NumGoroutine())
			stats.AddRow("", "")

			fmt.Fprintln(writer, stats.String())
			writer.Flush()
			time.Sleep(time.Millisecond * 250)
		}
	}
}

func (c *Crawl) getCrawlState() (state string) {
	if c.Finished.Get() {
		return "finishing"
	}

	if c.Paused.Get() {
		return "paused"
	}

	return "running"
}
