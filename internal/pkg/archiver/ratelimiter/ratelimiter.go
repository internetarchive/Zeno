package ratelimiter

import (
	"math"
	"sync"
	"time"
)

const (
	// Minimum refill rate (tokens per second) to avoid completely stalling.
	minRefillRate = 0.5

	// Maximum penalty duration.
	maxPenaltyDuration = 30 * time.Second

	// Base penalty duration for 429 errors.
	basePenaltyDuration = 5 * time.Second

	// Recovery factor controls how fast we restore the refill rate.
	recoveryFactor = 0.1
)

// tokenBucket implements a token bucket with penalty and recovery.
type tokenBucket struct {
	mu           sync.Mutex
	tokens       float64   // current tokens available
	capacity     float64   // maximum tokens in the bucket
	refillRate   float64   // current tokens per second
	idealRate    float64   // the target refill rate under good conditions
	lastRefill   time.Time // last time the bucket was refilled
	penaltyUntil time.Time // if set, no tokens will be refilled until this time
	failureCount int       // consecutive failure counter for exponential backoff

	// nowFunc is used to fetch the current time; it defaults to time.Now,
	// but can be overridden for testing.
	nowFunc func() time.Time
}

// newTokenBucket returns a new token bucket with the given capacity and refill rate.
func newTokenBucket(capacity, refillRate float64) *tokenBucket {
	now := time.Now()
	return &tokenBucket{
		tokens:     capacity,
		capacity:   capacity,
		refillRate: refillRate,
		idealRate:  refillRate,
		lastRefill: now,
		nowFunc:    time.Now,
	}
}

// Wait blocks until a token is available.
func (tb *tokenBucket) Wait() {
	for {
		tb.mu.Lock()
		tb.refill()
		if tb.tokens >= 1 {
			tb.tokens--
			tb.mu.Unlock()
			return
		}
		tb.mu.Unlock()
		time.Sleep(50 * time.Millisecond) // adjust as needed
	}
}

// refill adds tokens to the bucket based on the time elapsed.
func (tb *tokenBucket) refill() {
	now := tb.nowFunc()

	// If we're in a penalty period, don't refill tokens.
	if now.Before(tb.penaltyUntil) {
		return
	}

	// Calculate the elapsed time since the last refill or penalty.
	// If the penalty period is active, use that as the time.
	// This prevents the tokens from being refilled too much after recovering from a penalty.
	lastRefillOrPenaltyUntil := tb.lastRefill
	if tb.penaltyUntil.After(tb.lastRefill) {
		lastRefillOrPenaltyUntil = tb.penaltyUntil
	}
	elapsed := now.Sub(lastRefillOrPenaltyUntil).Seconds()

	if elapsed > 0 {
		tb.tokens = math.Min(tb.capacity, tb.tokens+elapsed*tb.refillRate)
		tb.lastRefill = now
	}
}
