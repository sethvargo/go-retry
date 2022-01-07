package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sethvargo/go-retry"
)

func TestCancel(t *testing.T) {
	for i := 0; i < 100000; i++ {
		ctx, cancel := context.WithCancel(context.Background())

		calls := 0
		rf := func(ctx context.Context) error {
			calls++
			// Never succeed.
			// Always return a RetryableError
			return retry.RetryableError(errors.New("nope"))
		}

		const delay time.Duration = time.Millisecond
		b := retry.NewConstant(delay)

		const maxRetries = 5
		b = retry.WithMaxRetries(maxRetries, b)

		const jitter time.Duration = 5 * time.Millisecond
		b = retry.WithJitter(jitter, b)

		// Here we cancel the Context *before* the call to Do
		cancel()
		retry.Do(ctx, b, rf)

		if calls > 1 {
			t.Errorf("rf was called %d times instead of 0 or 1", calls)
		}
	}
}
