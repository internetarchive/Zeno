// Package stats provides a way to store and display statistics about the crawl.
// This package can be used as a realiable source of truth for the current state of the crawl.
package stats

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CorentinB/warc"
	"github.com/dustin/go-humanize"
	"github.com/gosuri/uilive"
	"github.com/gosuri/uitable"
)

type Runner struct {
	stopChan chan struct{}
	doneChan chan struct{}
	data     *data

	// Config booleans
	handoverUsed    bool
	localDedupeUsed bool
	cdxDedupeUsed   bool
}

type Config struct {
	HandoverUsed    bool
	LocalDedupeUsed bool
	CDXDedupeUsed   bool
}

var (
	once          sync.Once
	initialized   *atomic.Bool
	packageRunner *Runner
)

// Init initializes the stats package
//
// Returns:
// - bool: true if the stats package was initialized, false otherwise meaning that stats was already initialized
func Init(config *Config) bool {
	if initialized.Load() {
		return false
	}

	var successfullyInit = false

	once.Do(func() {
		data := initStatsData()
		packageRunner = &Runner{
			stopChan:        make(chan struct{}),
			doneChan:        make(chan struct{}),
			data:            data,
			handoverUsed:    config.HandoverUsed,
			localDedupeUsed: config.LocalDedupeUsed,
			cdxDedupeUsed:   config.CDXDedupeUsed,
		}

		successfullyInit = initialized.CompareAndSwap(false, true)
	})

	return successfullyInit
}

// Printer starts the stats printer.
// Preferably run this in a goroutine.
// Is controlled by the StopChan channel.
func Printer() {
	var stats *uitable.Table
	var m runtime.MemStats

	writer := uilive.New()
	writer.Start()

	for {
		select {
		case <-packageRunner.stopChan:
			writer.Stop()
			packageRunner.doneChan <- struct{}{}
			return
		default:
			runtime.ReadMemStats(&m)

			stats = uitable.New()
			stats.MaxColWidth = 80
			stats.Wrap = true

			crawledSeeds := GetCrawledSeeds()
			crawledAssets := GetCrawledAssets()

			queueStats := GetQueueStats()

			stats.AddRow("", "")
			stats.AddRow("  - Job:", GetJob())
			stats.AddRow("  - State:", GetCrawlState())
			stats.AddRow("  - Active workers:", fmt.Sprintf("%d/%d", GetActiveWorkers(), GetTotalWorkers()))
			stats.AddRow("  - URI/s:", GetURIPerSecond())
			stats.AddRow("  - Items in queue:", queueStats.TotalElements)
			stats.AddRow("  - Hosts in queue:", queueStats.UniqueHosts)
			if packageRunner.handoverUsed {
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

			if packageRunner.localDedupeUsed {
				stats.AddRow("  - Deduped (local):", humanize.Bytes(uint64(warc.LocalDedupeTotal.Value())))
			}

			if packageRunner.cdxDedupeUsed {
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

// Stop stops the stats printer.
// Blocks until the printer is stopped.
func Stop() {
	r.stopChan <- struct{}{}
	<-r.doneChan
}
