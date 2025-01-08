package ringbuffer

import (
	"math/bits"
	"sync/atomic"
)

// MP1COverwritingRingBuffer is a multi-producer, single-consumer ring buffer
// with a fixed size that overwrites the oldest item when full.
type MP1COverwritingRingBuffer[T any] struct {
	items []atomic.Value // ring storage of generic type T
	size  uint64         // always a power of two
	mask  uint64

	// tail: total number of items "reserved" by producers so far
	tail atomic.Uint64

	// head: total number of items consumed so far (or forcibly advanced)
	head atomic.Uint64
}

// NewMP1COverwritingRingBuffer creates a ring buffer with capacity at least 'capacity',
// rounding up to the next power of two, so the ring won't grow infinitely.
func NewMP1COverwritingRingBuffer[T any](capacity uint64) *MP1COverwritingRingBuffer[T] {
	size := nextPowerOfTwo(capacity)
	mask := size - 1

	rb := &MP1COverwritingRingBuffer[T]{
		items: make([]atomic.Value, size),
		size:  size,
		mask:  mask,
	}
	rb.head.Store(0)
	rb.tail.Store(0)
	return rb
}

// nextPowerOfTwo rounds n up to the nearest power of two.
// This ensures index calculations are fast (using & mask).
func nextPowerOfTwo(n uint64) uint64 {
	if n < 2 {
		return 2
	}
	return 1 << (64 - bits.LeadingZeros64(n-1))
}

// Enqueue writes 'val' into the ring buffer, overwriting the oldest entry if necessary.
// It never fails or blocks; producers always succeed.
func (rb *MP1COverwritingRingBuffer[T]) Enqueue(val T) {
	for {
		oldTail := rb.tail.Load()
		oldHead := rb.head.Load()

		// If we appear "full", forcibly advance head by 1 (overwriting oldest).
		if oldTail-oldHead >= rb.size {
			// Attempt to increment head by 1.
			// If CAS fails, we retry the entire loop.
			if !rb.head.CompareAndSwap(oldHead, oldHead+1) {
				continue
			}
		}

		// Reserve the next slot via CAS on tail
		if rb.tail.CompareAndSwap(oldTail, oldTail+1) {
			// We have claimed index 'oldTail & rb.mask'.
			idx := oldTail & rb.mask
			rb.items[idx].Store(val)
			return
		}
		// If CAS fails, another producer advanced tail first; retry.
	}
}

// DumpN reads up to 'maxCount' items in a single batch.
// Returns a slice of length <= maxCount. If empty, returns nil.
//
// This is a single-consumer operation; only one goroutine should call DumpN.
func (rb *MP1COverwritingRingBuffer[T]) DumpN(maxCount uint64) []T {
	var zero T

	for {
		oldHead := rb.head.Load()
		oldTail := rb.tail.Load()

		// If buffer is empty
		if oldHead == oldTail {
			return nil
		}

		// Number of items currently available
		available := oldTail - oldHead
		n := available
		if n > maxCount {
			n = maxCount
		}

		// Copy out up to n items
		out := make([]T, 0, n)
		for i := uint64(0); i < n; i++ {
			idx := (oldHead + i) & rb.mask
			val := rb.items[idx].Load()
			typedVal, _ := val.(T)
			out = append(out, typedVal)
		}

		// Try to consume all n items at once
		if rb.head.CompareAndSwap(oldHead, oldHead+n) {
			// (Optional) Zero out the consumed slots for GC or security reasons
			for i := uint64(0); i < n; i++ {
				idx := (oldHead + i) & rb.mask
				rb.items[idx].Store(zero)
			}
			return out
		}
		// If CAS fails, a producer forcibly advanced head or there's a race;
		// retry with updated values.
	}
}
