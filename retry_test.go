package retry_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/sethvargo/go-retry"
)

func TestRetryableError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		in      error
		wantNil bool
		wantMsg string
	}{
		{
			name:    "nil",
			in:      nil,
			wantNil: true,
		},
		{
			name:    "wraps_error",
			in:      errors.New("oops"),
			wantMsg: "retryable: oops",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := retry.RetryableError(tc.in)
			if tc.wantNil {
				if err != nil {
					t.Errorf("expected nil, got %v", err)
				}
				return
			}

			if got := err.Error(); got != tc.wantMsg {
				t.Errorf("expected %q to be %q", got, tc.wantMsg)
			}
			if !errors.Is(err, tc.in) {
				t.Errorf("expected %v to wrap %v", err, tc.in)
			}
		})
	}
}

func TestDoValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		failUntil int
		want      string
	}{
		{
			name:      "returns_value",
			failUntil: 1,
			want:      "foo",
		},
		{
			name:      "retries_then_returns_value",
			failUntil: 3,
			want:      "foo",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			b := retry.WithMaxRetries(5, retry.BackoffFunc(func() (time.Duration, bool) {
				return 1 * time.Nanosecond, false
			}))

			calls := 0
			v, err := retry.DoValue(ctx, b, func(_ context.Context) (string, error) {
				calls++
				if calls < tc.failUntil {
					return "", retry.RetryableError(errors.New("retry"))
				}
				return tc.want, nil
			})
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if v != tc.want {
				t.Errorf("expected %q to be %q", v, tc.want)
			}
			if calls != tc.failUntil {
				t.Errorf("expected %d calls, got %d", calls, tc.failUntil)
			}
		})
	}
}

func TestDo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		maxRetries uint64
		fn         func() error
		wantErr    bool
		wantErrIs  error
		wantCalls  int
	}{
		{
			name:       "exit_on_max_attempt",
			maxRetries: 3,
			fn:         func() error { return retry.RetryableError(fmt.Errorf("oops")) },
			wantErr:    true,
			wantCalls:  4,
		},
		{
			name:       "exit_on_non_retryable",
			maxRetries: 3,
			fn:         func() error { return fmt.Errorf("oops") },
			wantErr:    true,
			wantCalls:  1,
		},
		{
			name:       "unwraps",
			maxRetries: 1,
			fn:         func() error { return retry.RetryableError(io.EOF) },
			wantErr:    true,
			wantErrIs:  io.EOF,
			wantCalls:  2,
		},
		{
			name:       "exit_no_error",
			maxRetries: 3,
			fn:         func() error { return nil },
			wantErr:    false,
			wantCalls:  1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			b := retry.WithMaxRetries(tc.maxRetries, retry.BackoffFunc(func() (time.Duration, bool) {
				return 1 * time.Nanosecond, false
			}))

			calls := 0
			err := retry.Do(ctx, b, func(_ context.Context) error {
				calls++
				return tc.fn()
			})

			if tc.wantErr && err == nil {
				t.Fatal("expected err")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if tc.wantErrIs != nil && !errors.Is(err, tc.wantErrIs) {
				t.Errorf("expected %v to be %v", err, tc.wantErrIs)
			}
			if calls != tc.wantCalls {
				t.Errorf("expected %d calls, got %d", calls, tc.wantCalls)
			}
		})
	}

	t.Run("context_canceled", func(t *testing.T) {
		t.Parallel()

		b := retry.BackoffFunc(func() (time.Duration, bool) {
			return 5 * time.Second, false
		})

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := retry.Do(ctx, b, func(_ context.Context) error {
			return retry.RetryableError(fmt.Errorf("oops"))
		})
		if got, want := err, context.DeadlineExceeded; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	})

	t.Run("deadline_exceeded", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		b := retry.NewConstant(20 * time.Millisecond)

		start := time.Now()
		err := retry.Do(ctx, b, func(_ context.Context) error {
			return retry.RetryableError(fmt.Errorf("oops"))
		})
		if err == nil {
			t.Fatal("expected err")
		}
		if got, want := err, context.DeadlineExceeded; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
		if got, want := time.Since(start), 15*time.Millisecond; got > want {
			t.Errorf("expected %s to be less than %s", got, want)
		}
	})

	t.Run("canceled_before_first_call", func(t *testing.T) {
		for range 100_000 {
			ctx, cancel := context.WithCancel(context.Background())

			calls := 0
			b := retry.WithJitter(5*time.Millisecond, retry.WithMaxRetries(5, retry.NewConstant(1*time.Millisecond)))

			cancel()
			retry.Do(ctx, b, func(_ context.Context) error {
				calls++
				return retry.RetryableError(errors.New("nope"))
			})

			if calls > 1 {
				t.Errorf("rf was called %d times instead of 0 or 1", calls)
			}
		}
	})
}

func ExampleDo_simple() {
	ctx := context.Background()

	b := retry.NewFibonacci(1 * time.Nanosecond)

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

	b := retry.NewFibonacci(1 * time.Nanosecond)

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

func ExampleDoValue() {
	ctx := context.Background()

	b := retry.NewFibonacci(1 * time.Nanosecond)

	body, err := retry.DoValue(ctx, retry.WithMaxRetries(3, b), func(ctx context.Context) ([]byte, error) {
		resp, err := http.Get("https://google.com/")
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		switch resp.StatusCode / 100 {
		case 4:
			return nil, fmt.Errorf("bad response: %v", resp.StatusCode)
		case 5:
			return nil, retry.RetryableError(fmt.Errorf("bad response: %v", resp.StatusCode))
		default:
			b, _ := io.ReadAll(resp.Body)
			return b, nil
		}
	})
	if err != nil {
		// handle error
	}
	_ = body
}
