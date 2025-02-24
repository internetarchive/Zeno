package ratelimiter

import (
	"context"
	"math"
	"sync"
	"time"
)

// managedBucket wraps a TokenBucket with usage info for LFU eviction.
type managedBucket struct {
	bucket     *tokenBucket
	usageCount int       // number of times this bucket has been used
	lastAccess time.Time // last time the bucket was accessed
}

// BucketManager manages token buckets keyed by host.
type BucketManager struct {
	mu          sync.Mutex
	buckets     map[string]*managedBucket
	maxBuckets  int           // maximum number of buckets allowed
	capacity    float64       // default bucket capacity
	refillRate  float64       // default refill rate for new buckets
	cleanupFreq time.Duration // how often to run cleanup of stale buckets
	done        chan struct{} // signal to close the cleanup loop
	ctx         context.Context
}

// NewBucketManager creates a new BucketManager. The manager will automatically
// create a token bucket for each new host and evict the least frequently used
// buckets if maxBuckets is exceeded.
func NewBucketManager(ctx context.Context, maxBuckets int, capacity, refillRate float64, cleanupFreq time.Duration) *BucketManager {
	bm := &BucketManager{
		buckets:     make(map[string]*managedBucket),
		maxBuckets:  maxBuckets,
		capacity:    capacity,
		refillRate:  refillRate,
		cleanupFreq: cleanupFreq,
		done:        make(chan struct{}),
		ctx:         ctx,
	}

	go bm.cleanupLoop() // start periodic cleanup of stale buckets

	return bm
}

// Close stops the BucketManager's cleanupLoop to prevent goroutine leaks.
func (bm *BucketManager) Close() {
	close(bm.done)
}

// getBucket returns the managedBucket for the host, creating it if needed.
// It also updates usage stats.
func (bm *BucketManager) getBucket(host string) *managedBucket {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if mb, ok := bm.buckets[host]; ok {
		mb.usageCount++
		mb.lastAccess = time.Now()
		return mb
	}

	// If we have reached the limit, evict the LFU bucket.
	if len(bm.buckets) >= bm.maxBuckets {
		bm.evictLFU()
	}

	tb := newTokenBucket(bm.capacity, bm.refillRate)
	mb := &managedBucket{
		bucket:     tb,
		usageCount: 1,
		lastAccess: time.Now(),
	}
	bm.buckets[host] = mb
	return mb
}

// evictLFU removes the bucket with the lowest usageCount.
func (bm *BucketManager) evictLFU() {
	var lfuKey string
	lfuUsage := math.MaxInt32
	for key, mb := range bm.buckets {
		if mb.usageCount < lfuUsage {
			lfuUsage = mb.usageCount
			lfuKey = key
		}
	}
	if lfuKey != "" {
		delete(bm.buckets, lfuKey)
	}
}

// Wait blocks until a token is available for the given host.
func (bm *BucketManager) Wait(host string) {
	mb := bm.getBucket(host)
	mb.bucket.Wait()
}

// AdjustOnFailure applies failure adjustments for the given host's bucket.
func (bm *BucketManager) AdjustOnFailure(host string, statusCode int) {
	mb := bm.getBucket(host)
	mb.bucket.adjustOnFailure(statusCode)
}

// OnSuccess signals success for the given host's bucket.
func (bm *BucketManager) OnSuccess(host string) {
	mb := bm.getBucket(host)
	mb.bucket.onSuccess()
}

// cleanupLoop runs periodically to remove buckets that haven't been accessed
// for a period longer than cleanupFreq.
func (bm *BucketManager) cleanupLoop() {
	ticker := time.NewTicker(bm.cleanupFreq)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			bm.mu.Lock()
			now := time.Now()
			for host, mb := range bm.buckets {
				if now.Sub(mb.lastAccess) > bm.cleanupFreq {
					delete(bm.buckets, host)
				}
			}
			bm.mu.Unlock()
		case <-bm.done:
			return
		case <-bm.ctx.Done():
			return
		}
	}
}
