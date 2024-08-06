package stats

import (
	"sync/atomic"

	"github.com/internetarchive/Zeno/internal/queue"
)

// Data is a struct that holds the data to be displayed in the stats table
// NEVER ACCESS THE FIELDS DIRECTLY, ALWAYS USE THE GETTERS AND SETTERS
type data struct {
	job           *atomic.Value
	crawlState    *atomic.Value
	crawledSeeds  uint64
	crawledAssets uint64
	queueStats    *atomic.Value
	uriPerSecond  uint64
}

func initStatsData() *data {
	return &data{
		job:        new(atomic.Value),
		crawlState: new(atomic.Value),
		queueStats: new(atomic.Value),
	}
}

/////////////////////////////////
// Setters and Getters for job //
/////////////////////////////////

// SetJob sets the job name
func (r *Runner) SetJob(job string) {
	r.data.job.Store(job)
}

// GetJob returns the job name
func (r *Runner) GetJob() string {
	return r.data.job.Load().(string)
}

////////////////////////////////////////
// Setters and Getters for crawlState //
////////////////////////////////////////

// SetCrawlState sets the crawl state
func (r *Runner) SetCrawlState(state string) {
	r.data.crawlState.Store(state)
}

// GetCrawlState returns the crawl state
func (r *Runner) GetCrawlState() string {
	return r.data.crawlState.Load().(string)
}

//////////////////////////////////////////
// Setters and Getters for crawledSeeds //
//////////////////////////////////////////

// SetCrawledSeeds sets the number of crawled seeds
func (r *Runner) SetCrawledSeeds(seeds uint64) {
	atomic.StoreUint64(&r.data.crawledSeeds, seeds)
}

// GetCrawledSeeds returns the number of crawled seeds
func (r *Runner) GetCrawledSeeds() uint64 {
	return atomic.LoadUint64(&r.data.crawledSeeds)
}

///////////////////////////////////////////
// Setters and Getters for crawledAssets //
///////////////////////////////////////////

// SetCrawledAssets sets the number of crawled assets
func (r *Runner) SetCrawledAssets(assets uint64) {
	atomic.StoreUint64(&r.data.crawledAssets, assets)
}

// GetCrawledAssets returns the number of crawled assets
func (r *Runner) GetCrawledAssets() uint64 {
	return atomic.LoadUint64(&r.data.crawledAssets)
}

////////////////////////////////////////
// Setters and Getters for queueStats //
////////////////////////////////////////

// SetQueueStats sets the queue stats
func (r *Runner) SetQueueStats(stats *queue.QueueStats) {
	r.data.queueStats.Store(stats)
}

// GetQueueStats returns the queue stats
func (r *Runner) GetQueueStats() *queue.QueueStats {
	return r.data.queueStats.Load().(*queue.QueueStats)
}

//////////////////////////////////////////
// Setters and Getters for uriPerSecond //
//////////////////////////////////////////

// SetURIPerSecond sets the number of URIs per second
func (r *Runner) SetURIPerSecond(uriPerSecond uint64) {
	atomic.StoreUint64(&r.data.uriPerSecond, uriPerSecond)
}

// GetURIPerSecond returns the number of URIs per second
func (r *Runner) GetURIPerSecond() uint64 {
	return atomic.LoadUint64(&r.data.uriPerSecond)
}
