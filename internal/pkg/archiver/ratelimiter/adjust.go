package ratelimiter

import (
	"math"
	"time"
)

// AdjustOnFailure applies real-world adjustments based on the HTTP status code.
func (tb *TokenBucket) AdjustOnFailure(statusCode int) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := tb.nowFunc()

	switch {
	// For rate limiting errors, impose a penalty period.
	case statusCode == 429:
		tb.failureCount++
		penalty := time.Duration(float64(basePenaltyDuration) * math.Pow(2, float64(tb.failureCount-1)))
		if penalty > maxPenaltyDuration {
			penalty = maxPenaltyDuration
		}
		tb.penaltyUntil = now.Add(penalty)
		// Optionally, clear tokens to prevent immediate further requests.
		tb.tokens = 0

	// For server errors like 503 or 5xx, reduce the refill rate exponentially.
	case statusCode == 503 || statusCode >= 500:
		tb.failureCount++
		newRefillRate := tb.refillRate * math.Pow(0.5, float64(tb.failureCount))
		if newRefillRate < minRefillRate {
			newRefillRate = minRefillRate
		}
		tb.refillRate = newRefillRate
		tb.tokens = 0

	default:
		// For non-error status codes, do nothing.
	}
}

// OnSuccess should be called when a request succeeds.
// It gradually restores the refill rate and resets the failure count.
func (tb *TokenBucket) OnSuccess() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := tb.nowFunc()

	// Only adjust if the penalty period is over.
	if now.After(tb.penaltyUntil) {
		// Gradually restore the refill rate toward the ideal rate.
		if tb.refillRate < tb.idealRate {
			// Increase by a fraction of the difference.
			tb.refillRate += (tb.idealRate - tb.refillRate) * recoveryFactor
			// Avoid overshooting.
			if tb.refillRate > tb.idealRate {
				tb.refillRate = tb.idealRate
			}
		}
		// Optionally, decrement failureCount to slowly forget past errors.
		if tb.failureCount > 0 {
			tb.failureCount--
		}
	}
}
