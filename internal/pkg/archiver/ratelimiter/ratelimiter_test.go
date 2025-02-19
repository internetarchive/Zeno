package ratelimiter

import (
	"math"
	"testing"
	"time"
)

// TestNewTokenBucket verifies that a newly created bucket has the expected idealRate.
func TestNewTokenBucket(t *testing.T) {
	tb := NewTokenBucket(10, 5)
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
	tb := NewTokenBucket(1, 100)
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
	tb := NewTokenBucket(1, 5) // 5 tokens/sec refill rate
	tb.tokens = 0

	done := make(chan struct{})
	start := time.Now()
	go func() {
		tb.Wait()
		close(done)
	}()

	// Wait for at most 500ms (should be enough time for a token to be refilled).
	select {
	case <-done:
		elapsed := time.Since(start)
		// At a refill rate of 5 tokens per second, we expect a wait of roughly 200ms.
		if elapsed < 150*time.Millisecond {
			t.Errorf("Wait returned too quickly: %v", elapsed)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Wait did not return in expected time")
	}
}

// TestAdjustOnFailure429 tests that AdjustOnFailure sets a penalty period for a 429 error.
func TestAdjustOnFailure429(t *testing.T) {
	tb := NewTokenBucket(10, 5)
	tb.AdjustOnFailure(429)

	if tb.tokens != 0 {
		t.Errorf("expected tokens to be 0 after AdjustOnFailure(429), got %f", tb.tokens)
	}

	now := time.Now()
	if !tb.penaltyUntil.After(now) {
		t.Errorf("expected penaltyUntil to be in the future, got %v", tb.penaltyUntil)
	}

	if tb.failureCount != 1 {
		t.Errorf("expected failureCount to be 1, got %d", tb.failureCount)
	}
}

// TestAdjustOnFailure503 tests that AdjustOnFailure reduces the refill rate for a 503 error.
func TestAdjustOnFailure503(t *testing.T) {
	tb := NewTokenBucket(10, 5)
	tb.AdjustOnFailure(503)

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
	tb := NewTokenBucket(10, 5)
	// Simulate failures that reduce the refill rate.
	tb.AdjustOnFailure(503)
	tb.AdjustOnFailure(503)
	reducedRate := tb.refillRate

	if reducedRate >= tb.idealRate {
		t.Fatalf("expected reducedRate (%f) to be lower than idealRate (%f)", reducedRate, tb.idealRate)
	}

	// Wait until any penalty period is over.
	time.Sleep(100 * time.Millisecond)

	// Call OnSuccess repeatedly until the refill rate is near idealRate or we reach a maximum number of iterations.
	const maxIterations = 200
	iterations := 0
	for math.Abs(tb.refillRate-tb.idealRate) > 0.01 && iterations < maxIterations {
		tb.OnSuccess()
		iterations++
	}

	if math.Abs(tb.refillRate-tb.idealRate) > 0.01 {
		t.Errorf("expected refillRate to be near idealRate (%f), got %f after %d iterations", tb.idealRate, tb.refillRate, iterations)
	}
	if tb.failureCount < 0 {
		t.Errorf("expected failureCount to be non-negative, got %d", tb.failureCount)
	}
}
