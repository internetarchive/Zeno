package ringbuffer

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestBasic verifies basic enqueue/dump functionality under single-thread usage.
func TestBasic(t *testing.T) {
	rb := NewMP1COverwritingRingBuffer[int](4)

	// Enqueue some items
	rb.Enqueue(1)
	rb.Enqueue(2)
	rb.Enqueue(3)

	// Dump up to 2
	out := rb.DumpN(2)
	if len(out) != 2 {
		t.Errorf("DumpN(2) = %v items, want 2", len(out))
	}
	if out[0] != 1 || out[1] != 2 {
		t.Errorf("Dumped wrong values: got %v, want [1,2]", out)
	}

	// Dump up to 2 again
	out = rb.DumpN(2)
	if len(out) != 1 {
		t.Errorf("Expected only 1 item left, got %d", len(out))
	}
	if out[0] != 3 {
		t.Errorf("Got %v, want 3", out[0])
	}

	// Now buffer is empty
	out = rb.DumpN(2)
	if out != nil {
		t.Errorf("Expected empty nil slice, got %v", out)
	}
}

// TestNextPowerOfTwo verifies the nextPowerOfTwo function.
func TestNextPowerOfTwo(t *testing.T) {
	tests := []struct {
		input    uint64
		expected uint64
	}{
		{0, 2},
		{1, 2},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{6, 8},
		{7, 8},
		{8, 8},
		{9, 16},
		{15, 16},
		{16, 16},
		{17, 32},
		{31, 32},
		{32, 32},
		{33, 64},
		{63, 64},
		{64, 64},
		{65, 128},
		{127, 128},
		{128, 128},
		{129, 256},
		{255, 256},
		{256, 256},
		{257, 512},
		{511, 512},
		{512, 512},
		{513, 1024},
		{1023, 1024},
		{1024, 1024},
		{1025, 2048},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("nextPowerOfTwo(%d)", tt.input), func(t *testing.T) {
			if got := nextPowerOfTwo(tt.input); got != tt.expected {
				t.Errorf("nextPowerOfTwo(%d) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

// TestOverwrite verifies that old items are discarded if we exceed capacity.
func TestOverwrite(t *testing.T) {
	rb := NewMP1COverwritingRingBuffer[int](4)

	// Fill the buffer of size 4
	rb.Enqueue(1)
	rb.Enqueue(2)
	rb.Enqueue(3)
	rb.Enqueue(4)

	// Next enqueue should force overwrite
	rb.Enqueue(5)

	// Now the oldest item (1) should have been discarded
	// Let's dump up to 10 items
	out := rb.DumpN(10)
	// Expect items [2,3,4,5]
	if len(out) != 4 {
		t.Fatalf("Expected 4 items, got %d", len(out))
	}
	want := []int{2, 3, 4, 5}
	for i, v := range out {
		if want[i] != v {
			t.Errorf("Expected %d at index %d, got %d", want[i], i, v)
		}
	}
}

// TestHighVolume simulates multiple producers generating ~100k entries/s total,
// while a single consumer drains them in batches (DumpN(100)) every ~100ms.
//
// Use `go test -race -v` to ensure data-race detection.
func TestHighVolume(t *testing.T) {
	const (
		ringCapacity     = 1 << 14 // 16384 slots
		producerCount    = 10
		totalPerProducer = 20000 // total logs to produce per producer
		batchSize        = 100
		consumerPeriod   = 100 * time.Millisecond
	)

	rb := NewMP1COverwritingRingBuffer[int](ringCapacity)

	var wg sync.WaitGroup
	start := time.Now()

	// Start multiple producers
	wg.Add(producerCount)
	for p := 0; p < producerCount; p++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < totalPerProducer; i++ {
				// Here we do a simple Enqueue of an integer
				msg := id*1_000_000 + i // encode producer id + index
				rb.Enqueue(msg)

				// ~simulate 100k/s total
				// If each producer tries to produce ~10k/s for 10 producers => 100k/s
				// That is 1 item every 100 microseconds. We'll do a Sleep(50us..200us)
				time.Sleep(time.Microsecond * time.Duration(rand.Intn(150)+50))
			}
		}(p)
	}

	// Single consumer
	stopConsumer := make(chan struct{})
	consumeCount := int64(0)

	// We collect logs, but since it's overwriting, we won't see them all.
	// We'll just track how many we've read so far to confirm progress.
	go func() {
		ticker := time.NewTicker(consumerPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-stopConsumer:
				return
			case <-ticker.C:
				out := rb.DumpN(batchSize)
				if len(out) > 0 {
					atomic.AddInt64(&consumeCount, int64(len(out)))
					// For demonstration, let's just print the count
					t.Logf("Consumer got %d logs in this batch", len(out))
				}
			}
		}
	}()

	// Wait for all producers to finish
	wg.Wait()
	// Signal consumer to stop
	close(stopConsumer)

	took := time.Since(start)
	totalProduced := int64(producerCount * totalPerProducer)
	t.Logf("Producers done: produced %d messages in %v", totalProduced, took)

	finalConsumed := atomic.LoadInt64(&consumeCount)
	t.Logf("Total logs consumed (best effort) = %d", finalConsumed)

	// Because overwriting is allowed, finalConsumed <= totalProduced, possibly much less
	if finalConsumed <= 0 {
		t.Errorf("Consumer apparently got 0 logs, that shouldn't happen!")
	}
}

// BenchmarkSampling measures how effectively the ring buffer "samples" logs
// under high production rates (~100k/s) for various parameter sets.
//
// Run with: go test -bench=BenchmarkSampling -benchtime=5s -cpu=1,2,4 -v
func BenchmarkSampling(b *testing.B) {
	// We’ll try different ring sizes, batch sizes, and consumer intervals.
	ringSizes := []uint64{4096, 16384, 65536}
	batchSizes := []uint64{50, 100, 500}
	consumerIntervals := []time.Duration{50 * time.Millisecond, 100 * time.Millisecond}

	// For the benchmark, we won't iterate with b.N in the usual sense.
	// Instead, each sub-benchmark will run for a fixed time (e.g. 2s or 5s).
	// We'll measure how many logs are produced vs. consumed in that time.
	runDuration := 2 * time.Second

	for _, ringSize := range ringSizes {
		for _, batchSize := range batchSizes {
			for _, interval := range consumerIntervals {
				name := fmt.Sprintf("Ring%d_Batch%d_Interval%v", ringSize, batchSize, interval)
				b.Run(name, func(b *testing.B) {
					// We only want to measure the scenario once per sub-benchmark,
					// not repeated b.N times. So we do:
					b.StopTimer()
					// Setup
					rb := NewMP1COverwritingRingBuffer[int](ringSize)

					// We'll use some concurrency to approximate ~100k logs/s total.
					// Let’s define how many producers, each producing at ~some rate.
					producerCount := 8
					// We'll measure how many logs were actually produced:
					var producedCount int64
					// We'll measure how many logs the consumer read:
					var consumedCount int64

					// Start producers
					var wg sync.WaitGroup
					wg.Add(producerCount)
					// We'll run producers for "runDuration"
					producerStop := make(chan struct{})

					for p := 0; p < producerCount; p++ {
						go func(id int) {
							defer wg.Done()
							r := rand.New(rand.NewSource(time.Now().UnixNano()))
							for {
								select {
								case <-producerStop:
									return
								default:
									// Enqueue an integer (id+random)
									val := id*1_000_000 + r.Intn(100000)
									rb.Enqueue(val)
									atomic.AddInt64(&producedCount, 1)
									// Sleep ~10 microseconds => ~100k/s across 8 producers?
									// (8 producers * (1 / 10us) = 80k/s, tune as needed)
									time.Sleep(10 * time.Microsecond)
								}
							}
						}(p)
					}

					// Start consumer (single)
					consumerStop := make(chan struct{})
					go func() {
						ticker := time.NewTicker(interval)
						defer ticker.Stop()
						for {
							select {
							case <-consumerStop:
								return
							case <-ticker.C:
								out := rb.DumpN(batchSize)
								atomic.AddInt64(&consumedCount, int64(len(out)))
							}
						}
					}()

					// Now we actually "run" the scenario
					b.StartTimer()
					time.Sleep(runDuration)
					b.StopTimer()

					// Signal producers and consumer to stop
					close(producerStop)
					wg.Wait()
					close(consumerStop)

					// final measurement
					pCount := atomic.LoadInt64(&producedCount)
					cCount := atomic.LoadInt64(&consumedCount)

					// In many benchmarks, we might do b.SetBytes(...) or b.ReportMetric(...).
					// For a "sampling rate," let's do:
					samplingRate := float64(cCount) / float64(pCount+1) // +1 avoid /0
					logsPerSecondConsumed := float64(cCount) / runDuration.Seconds()

					// Print or record the results
					b.ReportAllocs() // show memory allocations
					b.ReportMetric(float64(pCount)/runDuration.Seconds(), "produced_ops/s")
					b.ReportMetric(logsPerSecondConsumed, "consumed_ops/s")
					b.ReportMetric(samplingRate, "sampling_ratio")
				})
			}
		}
	}
}
