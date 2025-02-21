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

// TokenBucket implements a token bucket with penalty and recovery.
type TokenBucket struct {
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

// NewTokenBucket returns a new token bucket with the given capacity and refill rate.
func NewTokenBucket(capacity, refillRate float64) *TokenBucket {
	now := time.Now()
	return &TokenBucket{
		tokens:     capacity,
		capacity:   capacity,
		refillRate: refillRate,
		idealRate:  refillRate,
		lastRefill: now,
		nowFunc:    time.Now,
	}
}

// Wait blocks until a token is available.
func (tb *TokenBucket) Wait() {
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
func (tb *TokenBucket) refill() {
	now := tb.nowFunc()

	// If we're in a penalty period, don't refill tokens.
	if now.Before(tb.penaltyUntil) {
		return
	}

	elapsed := now.Sub(tb.lastRefill).Seconds()
	if elapsed > 0 {
		tb.tokens = math.Min(tb.capacity, tb.tokens+elapsed*tb.refillRate)
		tb.lastRefill = now
	}
}
