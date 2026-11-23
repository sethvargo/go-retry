package retry_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"testing/synctest"
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
			b := retry.WithMaxRetries(5, retry.Backoff(func(uint64) (time.Duration, bool) {
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
				t.Errorf("expected %d to be %d", calls, tc.failUntil)
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
			b := retry.WithMaxRetries(tc.maxRetries, retry.Backoff(func(uint64) (time.Duration, bool) {
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
				t.Errorf("expected %d to be %d", calls, tc.wantCalls)
			}
		})
	}

	t.Run("reused_backoff", func(t *testing.T) {
		t.Parallel()

		b := retry.WithMaxRetries(2, retry.NewConstant(1*time.Nanosecond))

		for range 2 {
			calls := 0
			_ = retry.Do(t.Context(), b, func(_ context.Context) error {
				calls++
				return retry.RetryableError(errors.New("oops"))
			})
			if got, want := calls, 3; got != want {
				t.Errorf("expected %d to be %d", got, want)
			}
		}
	})

	t.Run("context_canceled", func(t *testing.T) {
		t.Parallel()

		b := retry.Backoff(func(uint64) (time.Duration, bool) {
			return 5 * time.Second, false
		})

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		errOops := errors.New("oops")
		err := retry.Do(ctx, b, func(_ context.Context) error {
			cancel()
			return retry.RetryableError(errOops)
		})
		if got, want := err, context.Canceled; !errors.Is(got, want) {
			t.Errorf("expected %v to be %v", got, want)
		}
		if got, want := err, errOops; !errors.Is(got, want) {
			t.Errorf("expected %v to be %v", got, want)
		}
	})

	t.Run("deadline_exceeded", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
			defer cancel()

			b := retry.NewConstant(20 * time.Millisecond)

			errOops := errors.New("oops")
			start := time.Now()
			err := retry.Do(ctx, b, func(_ context.Context) error {
				return retry.RetryableError(errOops)
			})
			if got, want := err, context.DeadlineExceeded; !errors.Is(got, want) {
				t.Errorf("expected %v to be %v", got, want)
			}
			if got, want := err, errOops; !errors.Is(got, want) {
				t.Errorf("expected %v to be %v", got, want)
			}
			if got, want := time.Since(start), 10*time.Millisecond; got != want {
				t.Errorf("expected %s to be %s", got, want)
			}
		})
	})

	t.Run("canceled_before_first_call", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		calls := 0
		err := retry.Do(ctx, retry.NewConstant(1*time.Millisecond), func(_ context.Context) error {
			calls++
			return retry.RetryableError(errors.New("nope"))
		})
		if got, want := err, context.Canceled; !errors.Is(got, want) {
			t.Errorf("expected %v to be %v", got, want)
		}
		if got, want := calls, 0; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
	})
}

func ExampleDo_simple() {
	ctx := context.Background()

	b := retry.NewFibonacci(1 * time.Nanosecond)

	i := 0
	err := retry.Do(ctx, retry.WithMaxRetries(3, b), func(ctx context.Context) error {
		fmt.Printf("%d\n", i)
		i++
		return retry.RetryableError(fmt.Errorf("oops"))
	})
	fmt.Println(err)

	// Output:
	// 0
	// 1
	// 2
	// 3
	// oops
}

func ExampleDo_timeout() {
	// To limit the total time spent retrying, set a deadline on the context.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := retry.Do(ctx, retry.NewConstant(10*time.Millisecond), func(_ context.Context) error {
		return retry.RetryableError(errors.New("database is down"))
	})

	// The error wraps both the context error and the last error from f.
	fmt.Println(errors.Is(err, context.DeadlineExceeded))
	fmt.Println(err)

	// Output:
	// true
	// context deadline exceeded
	// database is down
}

func ExampleDo_customRetry() {
	// The test server returns 503 for the first two requests and 200 after that.
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	b := retry.WithMaxRetries(3, retry.NewConstant(1*time.Millisecond))

	// Do retries only errors wrapped with RetryableError. This function wraps
	// errors for 5xx responses, so Do retries them, and returns 4xx errors as is.
	err := retry.Do(context.Background(), b, func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		switch resp.StatusCode / 100 {
		case 4:
			return fmt.Errorf("bad response: %d", resp.StatusCode)
		case 5:
			return retry.RetryableError(fmt.Errorf("bad response: %d", resp.StatusCode))
		default:
			return nil
		}
	})
	fmt.Println(err, requests.Load())

	// Output:
	// <nil> 3
}

func ExampleDoValue() {
	b := retry.WithMaxRetries(3, retry.NewConstant(1*time.Millisecond))

	attempts := 0
	val, err := retry.DoValue(context.Background(), b, func(_ context.Context) (string, error) {
		attempts++
		if attempts < 3 {
			return "", retry.RetryableError(errors.New("not ready"))
		}
		return "ready", nil
	})
	fmt.Println(val, err, attempts)

	// Output:
	// ready <nil> 3
}
