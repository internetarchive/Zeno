package stats

import "sync/atomic"

type mean struct {
	count uint64
	sum   uint64
}

func (m *mean) add(value uint64) {
	atomic.AddUint64(&m.count, 1)
	atomic.AddUint64(&m.sum, value)
}

func (m *mean) get() float64 {
	count := atomic.LoadUint64(&m.count)
	sum := atomic.LoadUint64(&m.sum)

	if count == 0 {
		return 0
	}

	return float64(sum) / float64(count)
}

func (m *mean) reset() {
	atomic.StoreUint64(&m.count, 0)
	atomic.StoreUint64(&m.sum, 0)
}
