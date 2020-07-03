package retry

import (
	"context"
	"fmt"
	"time"
)

// Constant is a wrapper around Retry that uses a constant backoff.
func Constant(ctx context.Context, t time.Duration, f RetryFunc) error {
	b, err := NewConstant(t)
	if err != nil {
		return err
	}
	return Do(ctx, b, f)
}

// NewConstant creates a new constant backoff using the value t. The wait time
// is the provided constant value.
func NewConstant(t time.Duration) (Backoff, error) {
	if t <= 0 {
		return nil, fmt.Errorf("t must be greater than 0")
	}

	return BackoffFunc(func() (time.Duration, bool) {
		return t, false
	}), nil
}
