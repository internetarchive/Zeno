package ratelimiter

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"
)

func TestWaitCreatesBucket(t *testing.T) {
	ctx := context.Background()
	bm := NewBucketManager(ctx, 10, 10, 5, 1*time.Second)
	defer bm.Close()

	// Call Wait for a new host.
	host := "example.com"
	bm.Wait(host)

	// Verify that the bucket was created.
	bm.mu.Lock()
	mb, ok := bm.buckets[host]
	bm.mu.Unlock()
	if !ok {
		t.Fatalf("expected bucket for host %s to be created", host)
	}
	if mb.bucket == nil {
		t.Fatalf("expected non-nil token bucket for host %s", host)
	}
}

func TestLFUEviction(t *testing.T) {
	ctx := context.Background()
	// Allow only 2 buckets.
	bm := NewBucketManager(ctx, 2, 10, 5, 1*time.Second)
	defer bm.Close()

	// Access three different hosts.
	bm.Wait("host1")
	bm.Wait("host2")
	// Increase usage for host1 to protect it from eviction.
	bm.Wait("host1")
	bm.Wait("host3")

	// Since maxBuckets is 2, one bucket should have been evicted.
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if len(bm.buckets) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(bm.buckets))
	}
	// Expect that "host2" is the least frequently used and has been evicted.
	if _, exists := bm.buckets["host2"]; exists {
		t.Errorf("expected host2 to be evicted, but it still exists")
	}
}

func TestCleanupLoopRemovesStaleBuckets(t *testing.T) {
	ctx := context.Background()
	// Use a short cleanup frequency.
	bm := NewBucketManager(ctx, 10, 10, 5, 100*time.Millisecond)
	defer bm.Close()

	// Create a bucket and record its last access time.
	host := "stalehost.com"
	bm.Wait(host)

	// Update lastAccess to simulate staleness.
	bm.mu.Lock()
	if mb, ok := bm.buckets[host]; ok {
		mb.lastAccess = time.Now().Add(-200 * time.Millisecond)
	}
	bm.mu.Unlock()

	// Wait for cleanup to run.
	time.Sleep(300 * time.Millisecond)

	bm.mu.Lock()
	_, exists := bm.buckets[host]
	bm.mu.Unlock()
	if exists {
		t.Errorf("expected stale bucket for host %s to be cleaned up", host)
	}
}

func TestContextCancellationStopsCleanupLoop(t *testing.T) {
	// Create a context that cancels quickly.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	bm := NewBucketManager(ctx, 10, 10, 5, 100*time.Millisecond)
	// Wait longer than the context timeout.
	time.Sleep(200 * time.Millisecond)

	// At this point, the cleanupLoop should have exited.
	select {
	case <-bm.ctx.Done():
		// Expected.
	default:
		t.Errorf("expected context to be canceled")
	}

	// Also calling Wait should still work (it recreates the bucket if needed).
	bm.Wait("anotherhost")
	cancel() // ensure context is canceled
	bm.Close()
}

func TestCloseStopsCleanupLoop(t *testing.T) {
	ctx := context.Background()
	bm := NewBucketManager(ctx, 10, 10, 5, 100*time.Millisecond)

	// Call Close and then wait a little to ensure cleanupLoop has stopped.
	bm.Close()
	time.Sleep(150 * time.Millisecond)

	// After Close, the done channel should be closed.
	select {
	case <-bm.done:
		// Expected.
	default:
		t.Errorf("expected done channel to be closed after Close()")
	}
}

func TestAdjustOnFailureAndOnSuccess(t *testing.T) {
	ctx := context.Background()
	bm := NewBucketManager(ctx, 10, 10, 5, 1*time.Second)
	defer bm.Close()

	host := "testhost.com"
	// Initially, the bucket should have no failures.
	bm.Wait(host)
	bm.mu.Lock()
	initialFailureCount := bm.buckets[host].bucket.failureCount
	bm.mu.Unlock()
	if initialFailureCount != 0 {
		t.Errorf("expected initial failureCount to be 0, got %d", initialFailureCount)
	}

	// Apply a 429 error.
	bm.AdjustOnFailure(host, 429)
	bm.mu.Lock()
	failureAfter429 := bm.buckets[host].bucket.failureCount
	refillRateAfter429 := bm.buckets[host].bucket.refillRate
	penaltyUntil := bm.buckets[host].bucket.penaltyUntil
	bm.mu.Unlock()
	if failureAfter429 != 1 {
		t.Errorf("expected failureCount to be 1 after 429, got %d", failureAfter429)
	}
	if bm.buckets[host].bucket.tokens != 0 {
		t.Errorf("expected tokens to be 0 after 429, got %f", bm.buckets[host].bucket.tokens)
	}

	// Call OnSuccess while penalty is still active; state should remain unchanged.
	bm.OnSuccess(host)
	bm.mu.Lock()
	failureDuringPenalty := bm.buckets[host].bucket.failureCount
	currentRefill := bm.buckets[host].bucket.refillRate
	bm.mu.Unlock()
	if failureDuringPenalty != failureAfter429 {
		t.Errorf("expected failureCount to remain %d during penalty, got %d", failureAfter429, failureDuringPenalty)
	}
	if math.Abs(currentRefill-refillRateAfter429) > 0.001 {
		t.Errorf("expected refillRate to remain %f during penalty, got %f", refillRateAfter429, currentRefill)
	}

	// Override the nowFunc to simulate that the penalty period has expired.
	bm.mu.Lock()
	bm.buckets[host].bucket.nowFunc = func() time.Time { return penaltyUntil.Add(1 * time.Millisecond) }
	bm.mu.Unlock()

	// Call OnSuccess repeatedly; now failureCount should decrease.
	for i := 0; i < 10; i++ {
		bm.OnSuccess(host)
	}

	bm.mu.Lock()
	finalFailure := bm.buckets[host].bucket.failureCount
	finalRefill := bm.buckets[host].bucket.refillRate
	idealRate := bm.buckets[host].bucket.idealRate
	bm.mu.Unlock()

	if finalFailure >= failureAfter429 {
		t.Errorf("expected failureCount to decrease after successful calls, got %d", finalFailure)
	}
	if finalRefill < refillRateAfter429 || finalRefill > idealRate {
		t.Errorf("expected refillRate to be restored toward idealRate, got %f (ideal %f)", finalRefill, idealRate)
	}
}

func TestConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	// Use a manager with larger capacity and faster refill to allow many concurrent calls.
	bm := NewBucketManager(ctx, 10, 100, 100, 1*time.Second)
	defer bm.Close()

	hosts := []string{"host1", "host2", "host3", "host4", "host5"}
	var wg sync.WaitGroup
	concurrentCalls := 50

	for i := 0; i < concurrentCalls; i++ {
		for _, host := range hosts {
			wg.Add(1)
			go func(h string) {
				defer wg.Done()
				bm.Wait(h)
			}(host)
		}
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines finished.
	case <-time.After(2 * time.Second):
		t.Error("concurrent Wait calls did not complete in time")
	}
}

func TestConcurrentSameHost(t *testing.T) {
	ctx := context.Background()
	// Use a high capacity and refill rate to avoid blocking.
	bm := NewBucketManager(ctx, 10, 100, 50, 1*time.Second)
	defer bm.Close()

	host := "concurrent-host.com"
	var wg sync.WaitGroup
	concurrentCalls := 100

	// Spawn many goroutines that call Wait for the same host.
	for i := 0; i < concurrentCalls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bm.Wait(host)
		}()
	}
	wg.Wait()

	// Check that there is only one bucket for the host.
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if mb, exists := bm.buckets[host]; !exists || mb == nil {
		t.Errorf("expected bucket for host %s to exist", host)
	}

	// Also verify that the map contains only one entry for the given host.
	bucketCount := 0
	for k := range bm.buckets {
		if k == host {
			bucketCount++
		}
	}
	if bucketCount != 1 {
		t.Errorf("expected exactly 1 bucket for host %s, got %d", host, bucketCount)
	}
}
