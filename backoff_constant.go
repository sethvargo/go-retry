package retry

import (
	"context"
	"time"
)

// Constant calls [Do] with [NewConstant](base). With no retry limit, it retries
// until f succeeds, f returns an error not marked with [RetryableError], or ctx
// is done.
func Constant(ctx context.Context, base time.Duration, f RetryFunc) error {
	return Do(ctx, NewConstant(base), f)
}

// NewConstant returns a backoff that always waits base. It panics if base is
// less than or equal to zero.
func NewConstant(base time.Duration) Backoff {
	if base <= 0 {
		panic("base must be greater than 0")
	}

	return func(uint64) (time.Duration, bool) {
		return base, false
	}
}
