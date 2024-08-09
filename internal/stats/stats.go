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
	liveStats *atomic.Bool
	stopChan  chan struct{}
	doneChan  chan struct{}
	data      *data

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
	initialized   = new(atomic.Bool)
	packageRunner *Runner
	startTime     time.Time
)

func init() {
	initialized.Store(false)
}

// Init initializes the stats package
//
// Returns:
// - bool: true if the stats package was initialized, false otherwise meaning that stats was already initialized
func Init(config *Config) bool {
	if initialized.Load() {
		return false
	}

	if config == nil {
		config = &Config{
			HandoverUsed:    false,
			LocalDedupeUsed: false,
			CDXDedupeUsed:   false,
		}
	}

	var successfullyInit = false

	once.Do(func() {
		data := initStatsData()
		packageRunner = &Runner{
			liveStats:       new(atomic.Bool),
			stopChan:        make(chan struct{}),
			doneChan:        make(chan struct{}),
			data:            data,
			handoverUsed:    config.HandoverUsed,
			localDedupeUsed: config.LocalDedupeUsed,
			cdxDedupeUsed:   config.CDXDedupeUsed,
		}

		packageRunner.liveStats.Store(false)

		successfullyInit = initialized.CompareAndSwap(false, true)
		if successfullyInit {
			startTime = time.Now()
		}
	})

	return successfullyInit
}

// IsInitialized returns true if the stats package has been initialized, false otherwise
func IsInitialized() bool {
	return initialized.Load()
}

// Reset resets without closing the stats package
// This is NOT INTENDED to be used in production code
func Reset() {
	if initialized.Load() && packageRunner != nil {
		close(packageRunner.stopChan)
		close(packageRunner.doneChan)
	}
	packageRunner = nil
	initialized.Store(false)
	once = sync.Once{}
}

// Printer starts the stats printer.
// Preferably run this in a goroutine.
// Is controlled by the StopChan channel.
func Printer() {
	var stats *uitable.Table
	var m runtime.MemStats

	writer := uilive.New()
	writer.Start()

	if !packageRunner.liveStats.CompareAndSwap(false, true) {
		return
	}

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

			stats.AddRow("", "")
			stats.AddRow("  - Job:", GetJob())
			stats.AddRow("  - State:", GetCrawlState())
			stats.AddRow("  - Active workers:", fmt.Sprintf("%d/%d", GetActiveWorkers(), GetTotalWorkers()))
			stats.AddRow("  - URI/s:", GetURIPerSecond())
			stats.AddRow("  - Items in queue:", GetQueueTotalElementsCount())
			stats.AddRow("  - Hosts in queue:", GetQueueUniqueHostsCount())
			if packageRunner.handoverUsed {
				stats.AddRow("  - Handover open:", GetHandoverOpen())
				stats.AddRow("  - Handover Get() success:", GetHandoverSuccessGetCount())
			}
			stats.AddRow("  - Queue empty bool state:", GetQueueEmpty())
			stats.AddRow("  - Can Enqueue:", GetCanEnqueue())
			stats.AddRow("  - Can Dequeue:", GetCanDequeue())
			stats.AddRow("  - Crawled total:", crawledSeeds+crawledAssets)
			stats.AddRow("  - Crawled seeds:", crawledSeeds)
			stats.AddRow("  - Crawled assets:", crawledAssets)
			stats.AddRow("  - WARC writing queue:", GetWARCWritingQueue())
			stats.AddRow("  - Data written:", humanize.Bytes(uint64(warc.DataTotal.Value())))

			if packageRunner.localDedupeUsed {
				stats.AddRow("  - Deduped (local):", humanize.Bytes(uint64(warc.LocalDedupeTotal.Value())))
			}

			if packageRunner.cdxDedupeUsed {
				stats.AddRow("  - Deduped (via CDX):", humanize.Bytes(uint64(warc.RemoteDedupeTotal.Value())))
			}

			stats.AddRow("", "")
			stats.AddRow("  - Elapsed time:", time.Since(startTime).String())
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
	if !packageRunner.liveStats.Load() {
		return
	}
	packageRunner.stopChan <- struct{}{}

	<-packageRunner.doneChan
}
