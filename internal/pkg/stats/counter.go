package stats

import (
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/internetarchive/Zeno/internal/pkg/config"
)

type counter struct {
	count uint64
}

func (c *counter) incr(step uint64) {
	atomic.AddUint64(&c.count, step)
}

func (c *counter) decr(step uint64) {
	atomic.AddUint64(&c.count, ^uint64(step-1))
}

func (c *counter) get() uint64 {
	return atomic.LoadUint64(&c.count)
}

func (c *counter) reset() {
	atomic.StoreUint64(&c.count, 0)
}

// A GaugedCounter is a atomic counter + Prometheus gauge
// The promGauge needs to be wired in the Init, although it is optional
// Add(i) and Done() mutate the counter and Prometheus gauge in one call
type GaugedCounter struct {
	counter
	promGauge *prometheus.GaugeVec
}

func (gc *GaugedCounter) Add(n uint64) {
	gc.incr(n)
	if gc.promGauge != nil {
		gc.promGauge.WithLabelValues(config.Get().JobPrometheus, hostname, version).Add(float64(n))
	}
}

func (gc *GaugedCounter) Done() {
	gc.decr(1)
	if gc.promGauge != nil {
		gc.promGauge.WithLabelValues(config.Get().JobPrometheus, hostname, version).Dec()
	}
}

func (gc *GaugedCounter) Value() uint64 {
	return gc.get()
}
