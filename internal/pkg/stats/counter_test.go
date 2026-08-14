package stats

import (
	"sync/atomic"
	"testing"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestCounter_Incr(t *testing.T) {
	c := &counter{}

	// Increment the counter by 1
	c.incr(1)
	if atomic.LoadUint64(&c.count) != 1 {
		t.Errorf("expected count to be 1, got %d", atomic.LoadUint64(&c.count))
	}

	// Increment the counter by 5
	c.incr(5)
	if atomic.LoadUint64(&c.count) != 6 {
		t.Errorf("expected count to be 6, got %d", atomic.LoadUint64(&c.count))
	}
}

func TestCounter_Decr(t *testing.T) {
	c := &counter{}

	// Increment the counter by 10
	c.incr(10)
	if atomic.LoadUint64(&c.count) != 10 {
		t.Errorf("expected count to be 10, got %d", atomic.LoadUint64(&c.count))
	}

	// Decrement the counter by 3
	c.decr(3)
	if atomic.LoadUint64(&c.count) != 7 {
		t.Errorf("expected count to be 7, got %d", atomic.LoadUint64(&c.count))
	}

	// Decrement the counter by 7
	c.decr(7)
	if atomic.LoadUint64(&c.count) != 0 {
		t.Errorf("expected count to be 0, got %d", atomic.LoadUint64(&c.count))
	}
}

func TestCounter_Get(t *testing.T) {
	c := &counter{}

	// Increment the counter by 4
	c.incr(4)
	if c.get() != 4 {
		t.Errorf("expected count to be 4, got %d", c.get())
	}

	// Decrement the counter by 2
	c.decr(2)
	if c.get() != 2 {
		t.Errorf("expected count to be 2, got %d", c.get())
	}
}

func TestCounter_Reset(t *testing.T) {
	c := &counter{}

	// Increment the counter by 8
	c.incr(8)
	if atomic.LoadUint64(&c.count) != 8 {
		t.Errorf("expected count to be 8, got %d", atomic.LoadUint64(&c.count))
	}

	// Reset the counter
	c.reset()
	if atomic.LoadUint64(&c.count) != 0 {
		t.Errorf("expected count to be 0 after reset, got %d", atomic.LoadUint64(&c.count))
	}
}

func TestGaugedCounter_AddDoneValue(t *testing.T) {
	gc := &GaugedCounter{}

	gc.Add(3)
	if gc.Value() != 3 {
		t.Errorf("expected 3, got %d", gc.Value())
	}

	gc.Done()
	if gc.Value() != 2 {
		t.Errorf("expected 2, got %d", gc.Value())
	}

	gc.Add(5)
	gc.Done()
	gc.Done()
	if gc.Value() != 5 {
		t.Errorf("expected 5, got %d", gc.Value())
	}
}

func TestGaugedCounter_NilPrometheus(t *testing.T) {
	gc := &GaugedCounter{}

	gc.Add(1)
	defer gc.Done()

	if gc.Value() != 1 {
		t.Errorf("expected 1, got %d", gc.Value())
	}
}

func TestGaugedCounter_WithPrometheus(t *testing.T) {
	config.Set(&config.Config{JobPrometheus: "testjob"})
	hostname = "testhost"
	version = "v0.0.0-test"

	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "test_gauged_counter", Help: "test"},
		[]string{"project", "hostname", "version"},
	)

	gc := &GaugedCounter{promGauge: gauge}

	gc.Add(3)
	if gc.Value() != 3 {
		t.Errorf("atomic: expected 3, got %d", gc.Value())
	}
	promVal := testutil.ToFloat64(gauge.WithLabelValues("testjob", "testhost", "v0.0.0-test"))
	if promVal != 3 {
		t.Errorf("prometheus: expected 3, got %f", promVal)
	}

	gc.Done()
	gc.Done()
	gc.Done()
	if gc.Value() != 0 {
		t.Errorf("atomic: expected 0, got %d", gc.Value())
	}
	promVal = testutil.ToFloat64(gauge.WithLabelValues("testjob", "testhost", "v0.0.0-test"))
	if promVal != 0 {
		t.Errorf("prometheus: expected 0, got %f", promVal)
	}
}

func TestGaugedCounter_PrometheusBatchAdd(t *testing.T) {
	config.Set(&config.Config{JobPrometheus: "testjob"})
	hostname = "testhost"
	version = "v0.0.0-test"

	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "test_gauged_counter_batch", Help: "test"},
		[]string{"project", "hostname", "version"},
	)

	gc := &GaugedCounter{promGauge: gauge}

	gc.Add(10)
	gc.Done()
	gc.Add(5)

	if gc.Value() != 14 {
		t.Errorf("atomic: expected 14, got %d", gc.Value())
	}
	promVal := testutil.ToFloat64(gauge.WithLabelValues("testjob", "testhost", "v0.0.0-test"))
	if promVal != 14 {
		t.Errorf("prometheus: expected 14, got %f", promVal)
	}
}
