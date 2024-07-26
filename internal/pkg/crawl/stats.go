package crawl

import (
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/CorentinB/warc"
	"github.com/dustin/go-humanize"
	"github.com/gosuri/uilive"
	"github.com/gosuri/uitable"
)

func (c *Crawl) printLiveStats() {
	var stats *uitable.Table
	var m runtime.MemStats

	writer := uilive.New()
	writer.Start()

	for {
		runtime.ReadMemStats(&m)

		stats = uitable.New()
		stats.MaxColWidth = 80
		stats.Wrap = true

		crawledSeeds := c.CrawledSeeds.Value()
		crawledAssets := c.CrawledAssets.Value()

		stats.AddRow("", "")
		stats.AddRow("  - Job:", c.Job)
		stats.AddRow("  - State:", c.getCrawlState())
		stats.AddRow("  - Active workers:", strconv.Itoa(int(c.ActiveWorkers.Value()))+"/"+strconv.Itoa(c.Workers.wpLen()))
		stats.AddRow("  - URI/s:", c.URIsPerSecond.Rate())
		stats.AddRow("  - Queued:", c.Queue.GetStats().TotalElements)
		stats.AddRow("  - Queue empty bool state:", c.Queue.Empty.Get())
		stats.AddRow("  - Can Enqueue:", c.Queue.CanEnqueue())
		stats.AddRow("  - Can Dequeue:", c.Queue.CanDequeue())
		stats.AddRow("  - Hosts in queue:", c.Queue.GetStats().UniqueHosts)
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
		time.Sleep(time.Second * 1)
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

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
