package retry

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

type exponentialBackoff struct {
	base    time.Duration
	attempt uint64
}

// Exponential is a wrapper around Retry that uses an exponential backoff. It's
// very efficient, but does not check for overflow, so ensure you bound the
// retry.
func Exponential(ctx context.Context, base time.Duration, f RetryFunc) error {
	b, err := NewExponential(base)
	if err != nil {
		return err
	}
	return Do(ctx, b, f)
}

// NewExponential creates a new exponential backoff using the starting value of
// base and doubling on each failure (1, 2, 4, 8, 16, 32, 64...), up to max.
// It's very efficient, but does not check for overflow, so ensure you bound the
// retry.
func NewExponential(base time.Duration) (Backoff, error) {
	if base <= 0 {
		return nil, fmt.Errorf("base must be greater than 0")
	}

	return &exponentialBackoff{
		base: base,
	}, nil
}

// Next implements Backoff. It is safe for concurrent use.
func (b *exponentialBackoff) Next() (time.Duration, bool) {
	return b.base << (atomic.AddUint64(&b.attempt, 1) - 1), false
}
