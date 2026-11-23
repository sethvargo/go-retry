package retry

import (
	"context"
	"math"
	"time"
)

// Exponential is a wrapper around [Do] that uses an exponential backoff. See
// [NewExponential].
func Exponential(ctx context.Context, base time.Duration, f RetryFunc) error {
	return Do(ctx, NewExponential(base), f)
}

// NewExponential creates a new exponential backoff using the starting value of
// base and doubling on each failure (1, 2, 4, 8, 16, 32, 64...).
//
// Once it overflows, the function constantly returns the maximum time.Duration
// for a 64-bit integer.
//
// It panics if base is less than or equal to zero.
func NewExponential(base time.Duration) Backoff {
	if base <= 0 {
		panic("base must be greater than 0")
	}

	return func(attempt uint64) (time.Duration, bool) {
		if base > math.MaxInt64>>attempt {
			return math.MaxInt64, false
		}
		return base << attempt, false
	}
}
