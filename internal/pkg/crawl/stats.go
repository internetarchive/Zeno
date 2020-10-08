package crawl

import (
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/gosuri/uilive"
	"github.com/gosuri/uitable"
	"github.com/mackerelio/go-osstat/memory"
	"github.com/sirupsen/logrus"
)

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func getMemory() string {
	memory, err := memory.Get()
	if err != nil {
		log.WithFields(logrus.Fields{
			"error": err,
		}).Warning("Unable to get memory usage")
		return "error/error"
	}

	return strconv.Itoa(int(bToMb(memory.Used))) + "/" + strconv.Itoa(int(bToMb(memory.Total))) + "MB"
}

func (c *Crawl) printLiveStats() {
	var stats *uitable.Table
	var m runtime.MemStats

	writer := uilive.New()
	writer.Start()

	for !c.Finished.Get() {
		runtime.ReadMemStats(&m)

		stats = uitable.New()
		stats.MaxColWidth = 80
		stats.Wrap = true

		stats.AddRow("", "")
		stats.AddRow("  - Job:", c.Job)
		stats.AddRow("  - URL/s:", c.URLsPerSecond.Rate())
		stats.AddRow("  - Crawled:", c.Crawled.Value())
		stats.AddRow("  - Queued:", c.Frontier.QueueCount.Value())
		stats.AddRow("", "")
		stats.AddRow("  - Allocated (heap):", bToMb(m.Alloc))
		stats.AddRow("  - Goroutines:", runtime.NumGoroutine())
		stats.AddRow("", "")

		fmt.Fprintln(writer, stats.String())
		writer.Flush()
		time.Sleep(time.Second * 1)
	}

	writer.Stop()
}
