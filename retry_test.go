package retry_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sethvargo/go-retry"
)

func TestRetryableError(t *testing.T) {
	t.Parallel()

	err := retry.RetryableError(fmt.Errorf("oops"))
	if got, want := err.Error(), "retryable: "; !strings.Contains(got, want) {
		t.Errorf("expected %v to contain %v", got, want)
	}
}

func TestDo(t *testing.T) {
	t.Parallel()

	t.Run("unwrapping_error", func(t *testing.T) {
		t.Parallel()

		b := retry.BackoffFunc(func() (time.Duration, bool) {
			return 1 * time.Nanosecond, true
		})
		cause := fmt.Errorf("oops")

		ctx := context.Background()
		err := retry.Do(ctx, retry.WithMaxRetries(1, b), func(_ context.Context) error {
			return retry.RetryableError(cause)
		})
		if err != cause {
			t.Errorf("expected %v to be %v", err, cause)
		}
	})

	t.Run("exit_on_max_attempt", func(t *testing.T) {
		t.Parallel()

		b := retry.BackoffFunc(func() (time.Duration, bool) {
			return 1 * time.Nanosecond, false
		})

		ctx := context.Background()
		var i int
		err := retry.Do(ctx, retry.WithMaxRetries(3, b), func(_ context.Context) error {
			i++
			return retry.RetryableError(fmt.Errorf("oops"))
		})
		if err == nil {
			t.Fatal("expected err")
		}

		// 1 + retries
		if got, want := i, 4; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	})

	t.Run("exit_on_non_retryable", func(t *testing.T) {
		t.Parallel()

		b := retry.BackoffFunc(func() (time.Duration, bool) {
			return 1 * time.Nanosecond, false
		})

		ctx := context.Background()
		var i int
		err := retry.Do(ctx, retry.WithMaxRetries(3, b), func(_ context.Context) error {
			i++
			return fmt.Errorf("oops") // not retryable
		})
		if err == nil {
			t.Fatal("expected err")
		}

		if got, want := i, 1; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	})

	t.Run("exit_no_error", func(t *testing.T) {
		t.Parallel()

		b := retry.BackoffFunc(func() (time.Duration, bool) {
			return 1 * time.Nanosecond, false
		})

		ctx := context.Background()
		var i int
		err := retry.Do(ctx, retry.WithMaxRetries(3, b), func(_ context.Context) error {
			i++
			return nil // no error
		})
		if err != nil {
			t.Fatal("expected no err")
		}

		if got, want := i, 1; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	})

	t.Run("context_canceled", func(t *testing.T) {
		t.Parallel()

		b := retry.BackoffFunc(func() (time.Duration, bool) {
			return 5 * time.Second, false
		})

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := retry.Do(ctx, b, func(_ context.Context) error {
			return retry.RetryableError(fmt.Errorf("oops")) // no error
		})
		if err != context.DeadlineExceeded {
			t.Errorf("expected %v to be %v", err, context.DeadlineExceeded)
		}
	})
}

func ExampleDo_simple() {
	ctx := context.Background()
	b, err := retry.NewFibonacci(1 * time.Nanosecond)
	if err != nil {
		// handle error
	}

	i := 0
	if err := retry.Do(ctx, retry.WithMaxRetries(3, b), func(ctx context.Context) error {
		fmt.Printf("%d\n", i)
		i++
		return retry.RetryableError(fmt.Errorf("oops"))
	}); err != nil {
		// handle error
	}

	// Output:
	// 0
	// 1
	// 2
	// 3
}

func ExampleDo_customRetry() {
	ctx := context.Background()
	b, err := retry.NewFibonacci(1 * time.Nanosecond)
	if err != nil {
		// handle error
	}

	// This example demonstrates selectively retrying specific errors. Only errors
	// wrapped with RetryableError are eligible to be retried.
	if err := retry.Do(ctx, retry.WithMaxRetries(3, b), func(ctx context.Context) error {
		resp, err := http.Get("https://google.com/")
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		switch resp.StatusCode / 100 {
		case 4:
			return fmt.Errorf("bad response: %v", resp.StatusCode)
		case 5:
			return retry.RetryableError(fmt.Errorf("bad response: %v", resp.StatusCode))
		default:
			return nil
		}
	}); err != nil {
		// handle error
	}
}
