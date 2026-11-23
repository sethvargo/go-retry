package retry

import (
	"context"
	"time"
)

// Constant is a wrapper around [Do] that uses a constant backoff. See
// [NewConstant].
func Constant(ctx context.Context, base time.Duration, f RetryFunc) error {
	return Do(ctx, NewConstant(base), f)
}

// NewConstant creates a new constant backoff that always waits base. It panics
// if base is less than or equal to zero.
func NewConstant(base time.Duration) Backoff {
	if base <= 0 {
		panic("base must be greater than 0")
	}

	return func(uint64) (time.Duration, bool) {
		return base, false
	}
}
