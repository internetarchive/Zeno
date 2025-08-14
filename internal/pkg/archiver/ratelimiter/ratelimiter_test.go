package ratelimiter

import (
	"math"
	"sync"
	"testing"
	"time"
)

// TestNewTokenBucket verifies that a newly created bucket has the expected idealRate.
func TestNewTokenBucket(t *testing.T) {
	tb := newTokenBucket(10, 5)
	if tb.idealRate != 5 {
		t.Fatalf("expected idealRate to be 5, got %f", tb.idealRate)
	}
	if tb.refillRate != 5 {
		t.Fatalf("expected refillRate to be 5, got %f", tb.refillRate)
	}
	if tb.tokens != 10 {
		t.Fatalf("expected tokens to be 10, got %f", tb.tokens)
	}
}

// TestWaitWithAvailableToken ensures that Wait consumes a token when one is available.
func TestWaitWithAvailableToken(t *testing.T) {
	tb := newTokenBucket(1, 100)

	// Ensure tokens are initially full.
	if tb.tokens != 1 {
		t.Fatalf("expected 1 token, got %f", tb.tokens)
	}

	// Call Wait and check that the token count decreases.
	tb.Wait()
	if tb.tokens != 0 {
		t.Errorf("expected 0 tokens after Wait, got %f", tb.tokens)
	}
}

// TestWaitBlocksUntilTokenIsAvailable tests that Wait will block if no tokens are available,
// and then return when tokens are refilled.
func TestWaitBlocksUntilTokenIsAvailable(t *testing.T) {
	tb := newTokenBucket(1, 5) // 5 tokens/sec refill rate
	// Override nowFunc to simulate time progression.
	baseTime := time.Now()
	tb.nowFunc = func() time.Time { return baseTime }
	tb.tokens = 0
	tb.lastRefill = baseTime

	done := make(chan struct{})
	go func() {
		// Advance time in a separate goroutine.
		for range 5 {
			time.Sleep(50 * time.Millisecond)
			baseTime = baseTime.Add(50 * time.Millisecond)
		}
		tb.Wait()
		close(done)
	}()

	select {
	case <-done:
		// success – Wait returned after simulated time progression
	case <-time.After(500 * time.Millisecond):
		t.Error("Wait did not return in expected time")
	}
}

// TestAdjustOnFailure429 tests that AdjustOnFailure sets a penalty period for a 429 error.
func TestAdjustOnFailure429(t *testing.T) {
	tb := newTokenBucket(10, 5)
	tb.adjustOnFailure(429)

	if tb.tokens != 0 {
		t.Errorf("expected tokens to be 0 after AdjustOnFailure(429), got %f", tb.tokens)
	}

	now := tb.nowFunc()
	if !tb.penaltyUntil.After(now) {
		t.Errorf("expected penaltyUntil to be in the future, got %v", tb.penaltyUntil)
	}

	if tb.failureCount != 1 {
		t.Errorf("expected failureCount to be 1, got %d", tb.failureCount)
	}
}

// TestAdjustOnFailure503 tests that AdjustOnFailure reduces the refill rate for a 503 error.
func TestAdjustOnFailure503(t *testing.T) {
	tb := newTokenBucket(10, 5)
	tb.adjustOnFailure(503)

	if tb.tokens != 0 {
		t.Errorf("expected tokens to be 0 after AdjustOnFailure(503), got %f", tb.tokens)
	}
	expectedRate := 5 * math.Pow(0.5, float64(tb.failureCount))
	if math.Abs(tb.refillRate-expectedRate) > 0.001 {
		t.Errorf("expected refillRate to be %f, got %f", expectedRate, tb.refillRate)
	}
}

// TestOnSuccess tests that OnSuccess gradually restores the refill rate and reduces the failureCount.
func TestOnSuccess(t *testing.T) {
	tb := newTokenBucket(10, 5)

	// Override nowFunc to control time progression.
	currentTime := time.Now()
	tb.nowFunc = func() time.Time { return currentTime }

	// Simulate failures that reduce the refill rate.
	tb.adjustOnFailure(503)
	tb.adjustOnFailure(503)
	reducedRate := tb.refillRate

	if reducedRate >= tb.idealRate {
		t.Fatalf("expected reducedRate (%f) to be lower than idealRate (%f)", reducedRate, tb.idealRate)
	}

	// Advance time beyond any penalty period.
	currentTime = currentTime.Add(1 * time.Second)

	// Call OnSuccess repeatedly until the refill rate is near idealRate or we reach max iterations.
	const maxIterations = 200
	iterations := 0
	for math.Abs(tb.refillRate-tb.idealRate) > 0.01 && iterations < maxIterations {
		tb.onSuccess()
		iterations++
		// simulate time progression between calls
		currentTime = currentTime.Add(10 * time.Millisecond)
	}

	if math.Abs(tb.refillRate-tb.idealRate) > 0.01 {
		t.Errorf("expected refillRate to be near idealRate (%f), got %f after %d iterations", tb.idealRate, tb.refillRate, iterations)
	}
	if tb.failureCount < 0 {
		t.Errorf("expected failureCount to be non-negative, got %d", tb.failureCount)
	}
}

// TestConcurrentWait tests the token bucket under concurrent usage.
func TestConcurrentWait(t *testing.T) {
	tb := newTokenBucket(5, 10)
	// Preload tokens.
	tb.tokens = 5

	var wg sync.WaitGroup
	concurrentCalls := 10
	results := make(chan struct{}, concurrentCalls)

	// Start concurrent goroutines that call Wait.
	for range concurrentCalls {
		wg.Go(func() {
			tb.Wait()
			results <- struct{}{}
		})
	}

	// Wait for all to finish (with a timeout safeguard).
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines finished.
	case <-time.After(1 * time.Second):
		t.Error("Concurrent Wait calls did not complete in time")
	}

	// The first 5 calls should have consumed the initial tokens; the others waited until refill.
	if len(results) != concurrentCalls {
		t.Errorf("expected %d results, got %d", concurrentCalls, len(results))
	}
}

// TestNonErrorStatusDoesNothing tests that AdjustOnFailure does not alter the state for non-error codes.
func TestNonErrorStatusDoesNothing(t *testing.T) {
	tb := newTokenBucket(10, 5)
	initialTokens := tb.tokens
	initialRate := tb.refillRate
	initialFailureCount := tb.failureCount

	tb.adjustOnFailure(200) // A non-error status

	if tb.tokens != initialTokens {
		t.Errorf("expected tokens to remain %f, got %f", initialTokens, tb.tokens)
	}
	if tb.refillRate != initialRate {
		t.Errorf("expected refillRate to remain %f, got %f", initialRate, tb.refillRate)
	}
	if tb.failureCount != initialFailureCount {
		t.Errorf("expected failureCount to remain %d, got %d", initialFailureCount, tb.failureCount)
	}
}

// TestOnSuccessDuringPenalty tests that OnSuccess does not restore refill rate if penalty period is still active.
func TestOnSuccessDuringPenalty(t *testing.T) {
	tb := newTokenBucket(10, 5)
	// Set a custom time function.
	baseTime := time.Now()
	tb.nowFunc = func() time.Time { return baseTime }

	// Force a penalty period via a 429 error.
	tb.adjustOnFailure(429)
	// Save state after failure.
	penaltyUntil := tb.penaltyUntil
	reducedRate := tb.refillRate
	failureCount := tb.failureCount

	// Advance time but not past the penalty.
	baseTime = baseTime.Add(2 * time.Second)

	// Call OnSuccess; since penalty is still active, state should remain unchanged.
	tb.onSuccess()

	if tb.refillRate != reducedRate {
		t.Errorf("expected refillRate to remain %f during active penalty, got %f", reducedRate, tb.refillRate)
	}
	if tb.failureCount != failureCount {
		t.Errorf("expected failureCount to remain %d during active penalty, got %d", failureCount, tb.failureCount)
	}
	if !tb.penaltyUntil.Equal(penaltyUntil) {
		t.Errorf("expected penaltyUntil to remain %v, got %v", penaltyUntil, tb.penaltyUntil)
	}
}
