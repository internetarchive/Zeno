package stats

import (
	"sync/atomic"
	"time"
)

type rate struct {
	total      atomic.Uint64
	count      atomic.Uint64
	lastCount  atomic.Uint64
	lastUpdate atomic.Int64
}

func (rps *rate) incr(step uint64) {
	rps.count.Add(step)
	rps.total.Add(step)
}

func (rps *rate) get() uint64 {
	now := time.Now().Unix()
	lastUpdate := rps.lastUpdate.Load()

	if now == lastUpdate {
		return rps.lastCount.Load()
	}

	currentCount := rps.count.Load()
	lastCount := rps.count.Swap(0)
	rps.lastCount.Store(lastCount)
	rps.lastUpdate.Store(now)

	return currentCount
}

func (rps *rate) getTotal() uint64 {
	return rps.total.Load()
}

func (rps *rate) reset() {
	rps.count.Store(0)
	rps.lastCount.Store(0)
	rps.lastUpdate.Store(0)
}
