package retry

import (
	"context"
	"math"
	"time"
)

// Exponential calls [Do] with [NewExponential](base). With no retry limit, it
// retries until f succeeds, f returns an error not marked with
// [RetryableError], or ctx is done.
func Exponential(ctx context.Context, base time.Duration, f RetryFunc) error {
	return Do(ctx, NewExponential(base), f)
}

// NewExponential returns a backoff that waits base, then doubles the wait on
// each retry (1, 2, 4, 8, 16, 32, 64...). Once the wait overflows, it stays at
// math.MaxInt64.
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
