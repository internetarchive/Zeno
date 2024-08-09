package stats

import (
	"sync/atomic"
	"time"

	"github.com/paulbellamy/ratecounter"
)

// Data is a struct that holds the data to be displayed in the stats table
// NEVER ACCESS THE FIELDS DIRECTLY, ALWAYS USE THE GETTERS AND SETTERS
type data struct {
	// Job name
	job *atomic.Value

	// Crawl
	crawlState    *atomic.Value
	crawledSeeds  *ratecounter.Counter
	crawledAssets *ratecounter.Counter
	uriPerSecond  *ratecounter.RateCounter

	// Workers
	activeWorkers *atomic.Int32
	totalWorkers  *atomic.Int32

	// Queue
	queue *queueStats

	// WARC
	warcWritingQueue *atomic.Int32
}

func initStatsData() *data {
	return &data{
		job:              new(atomic.Value),
		crawlState:       new(atomic.Value),
		crawledSeeds:     new(ratecounter.Counter),
		crawledAssets:    new(ratecounter.Counter),
		queue:            newQueueStats(),
		uriPerSecond:     ratecounter.NewRateCounter(1 * time.Second),
		activeWorkers:    new(atomic.Int32),
		totalWorkers:     new(atomic.Int32),
		warcWritingQueue: new(atomic.Int32),
	}
}

/////////////////////////////////
// Setters and Getters for job //
/////////////////////////////////

// SetJob sets the job name
func SetJob(job string) {
	packageRunner.data.job.Store(job)
}

// GetJob returns the job name
func GetJob() string {
	v := packageRunner.data.job.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

////////////////////////////////////////
// Setters and Getters for crawlState //
////////////////////////////////////////

// SetCrawlState sets the crawl state
func SetCrawlState(state string) {
	packageRunner.data.crawlState.Store(state)
}

// GetCrawlState returns the crawl state
func GetCrawlState() string {
	v := packageRunner.data.crawlState.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

//////////////////////////////////////////
// Setters and Getters for crawledSeeds //
//////////////////////////////////////////

// IncreaseCrawledSeeds increase the number of crawled seeds
func IncreaseCrawledSeeds(seeds int64) {
	packageRunner.data.crawledSeeds.Incr(seeds)
}

// GetCrawledSeeds returns the number of crawled seeds
func GetCrawledSeeds() int64 {
	return packageRunner.data.crawledSeeds.Value()
}

///////////////////////////////////////////
// Setters and Getters for crawledAssets //
///////////////////////////////////////////

// IncreaseCrawledAssets increase the number of crawled assets
func IncreaseCrawledAssets(assets int64) {
	packageRunner.data.crawledAssets.Incr(assets)
}

// GetCrawledAssets returns the number of crawled assets
func GetCrawledAssets() int64 {
	return packageRunner.data.crawledAssets.Value()
}

//////////////////////////////////////////
// Setters and Getters for uriPerSecond //
//////////////////////////////////////////

// IncreaseURIPerSecond sets the number of URIs per second
func IncreaseURIPerSecond(uriPerSecond int64) {
	packageRunner.data.uriPerSecond.Incr(uriPerSecond)
}

// GetURIPerSecond returns the number of URIs per second
func GetURIPerSecond() int64 {
	return packageRunner.data.uriPerSecond.Rate()
}

///////////////////////////////////////////
// Setters and Getters for activeWorkers //
///////////////////////////////////////////

// IncreaseActiveWorkers increase the number of active workers
func IncreaseActiveWorkers() {
	packageRunner.data.activeWorkers.Add(1)
}

// DecreaseActiveWorkers decrease the number of active workers
func DecreaseActiveWorkers() {
	packageRunner.data.activeWorkers.Add(-1)
}

// GetActiveWorkers returns the number of active workers
func GetActiveWorkers() int32 {
	return packageRunner.data.activeWorkers.Load()
}

//////////////////////////////////////////
// Setters and Getters for totalWorkers //
//////////////////////////////////////////

// IncreaseTotalWorkers increase the number of active workers
func IncreaseTotalWorkers() {
	packageRunner.data.totalWorkers.Add(1)
}

// DecreaseTotalWorkers decrease the number of active workers
func DecreaseTotalWorkers() {
	packageRunner.data.totalWorkers.Add(-1)
}

// GetTotalWorkers returns the number of active workers
func GetTotalWorkers() int32 {
	return packageRunner.data.totalWorkers.Load()
}

//////////////////////////////////
// Setters and Getters for WARC //
//////////////////////////////////

// SetWARCWritingQueue sets the number of items in the WARC writing queue
func SetWARCWritingQueue(count int32) {
	packageRunner.data.warcWritingQueue.Store(count)
}

// GetWARCWritingQueue returns the number of items in the WARC writing queue
func GetWARCWritingQueue() int32 {
	return packageRunner.data.warcWritingQueue.Load()
}