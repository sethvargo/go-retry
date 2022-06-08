package retry_test

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/sethvargo/go-retry"
)

func ExampleBackoffFunc() {
	ctx := context.Background()

	// Example backoff middleware that adds the provided duration t to the result.
	withShift := func(t time.Duration, next retry.Backoff) retry.BackoffFunc {
		return func(err error) (time.Duration, error) {
			delay, err := next.Next(err)
			if retry.IsStopped(delay) {
				return retry.Stop, err
			}
			return delay + t, err
		}
	}

	// Middlewrap wrap another backoff:
	b := retry.NewFibonacci(1 * time.Second)
	b = withShift(5*time.Second, b)

	if err := retry.Do(ctx, b, func(ctx context.Context) error {
		// Actual retry logic here
		return nil
	}); err != nil {
		// handle error
	}
}

func TestWithJitter(t *testing.T) {
	t.Parallel()

	for i := 0; i < 10_000; i++ {
		b := retry.WithJitter(250*time.Millisecond, false, retry.BackoffFunc(func(err error) (time.Duration, error) {
			return 1 * time.Second, err
		}))
		delay, _ := b.Next(nil)
		if retry.IsStopped(delay) {
			t.Errorf("should not stop")
		}

		if min, max := 750*time.Millisecond, 1250*time.Millisecond; delay < min || delay > max {
			t.Errorf("expected %v to be between %v and %v", delay, min, max)
		}
	}

	for i := 0; i < 10_000; i++ {
		b := retry.WithJitter(500*time.Millisecond, true, retry.BackoffFunc(func(err error) (time.Duration, error) {
			return 1 * time.Second, err
		}))
		delay, _ := b.Next(nil)
		if retry.IsStopped(delay) {
			t.Errorf("should not stop")
		}

		if min, max := 1000*time.Millisecond, 1500*time.Millisecond; delay < min || delay > max {
			t.Errorf("expected %v to be between %v and %v", delay, min, max)
		}
	}
}

func ExampleWithJitter() {
	ctx := context.Background()

	b := retry.NewFibonacci(1 * time.Second)
	b = retry.WithJitter(1*time.Second, false, b)

	if err := retry.Do(ctx, b, func(_ context.Context) error {
		// TODO: logic here
		return nil
	}); err != nil {
		// handle error
	}
}

func TestWithJitterPercent(t *testing.T) {
	t.Parallel()

	for i := 0; i < 10_000; i++ {
		b := retry.WithJitterPercent(5, false, retry.BackoffFunc(func(err error) (time.Duration, error) {
			return 1 * time.Second, err
		}))
		delay, _ := b.Next(nil)
		if retry.IsStopped(delay) {
			t.Errorf("should not stop")
		}

		if min, max := 950*time.Millisecond, 1050*time.Millisecond; delay < min || delay > max {
			t.Errorf("expected %v to be between %v and %v", delay, min, max)
		}
	}

	for i := 0; i < 10_000; i++ {
		b := retry.WithJitterPercent(5, true, retry.BackoffFunc(func(err error) (time.Duration, error) {
			return 1 * time.Second, err
		}))
		delay, _ := b.Next(nil)
		if retry.IsStopped(delay) {
			t.Errorf("should not stop")
		}

		if min, max := 1000*time.Millisecond, 1050*time.Millisecond; delay < min || delay > max {
			t.Errorf("expected %v to be between %v and %v", delay, min, max)
		}
	}
}

func ExampleWithJitterPercent() {
	ctx := context.Background()

	b := retry.NewFibonacci(1 * time.Second)
	b = retry.WithJitterPercent(5, false, b)

	if err := retry.Do(ctx, b, func(_ context.Context) error {
		// TODO: logic here
		return nil
	}); err != nil {
		// handle error
	}
}

func TestWithMaxRetries(t *testing.T) {
	t.Parallel()

	b := retry.WithMaxRetries(3, retry.BackoffFunc(func(err error) (time.Duration, error) {
		return 1 * time.Second, err
	}))

	// First 3 attempts succeed
	for i := 0; i < 3; i++ {
		delay, _ := b.Next(nil)
		if retry.IsStopped(delay) {
			t.Errorf("should not stop")
		}
		if delay != 1*time.Second {
			t.Errorf("expected %v to be %v", delay, 1*time.Second)
		}
	}

	// Now we stop
	delay, _ := b.Next(nil)
	if delay >= 0 {
		t.Errorf("should stop")
	}
}

func ExampleWithMaxRetries() {
	ctx := context.Background()

	b := retry.NewFibonacci(1 * time.Second)
	b = retry.WithMaxRetries(3, b)

	if err := retry.Do(ctx, b, func(_ context.Context) error {
		// TODO: logic here
		return nil
	}); err != nil {
		// handle error
	}
}

func TestWithCappedDuration(t *testing.T) {
	t.Parallel()

	b := retry.WithCappedDuration(3*time.Second, retry.BackoffFunc(func(err error) (time.Duration, error) {
		return 5 * time.Second, err
	}))

	delay, _ := b.Next(nil)
	if retry.IsStopped(delay) {
		t.Errorf("should not stop")
	}
	if delay != 3*time.Second {
		t.Errorf("expected %v to be %v", delay, 3*time.Second)
	}
}

func ExampleWithCappedDuration() {
	ctx := context.Background()

	b := retry.NewFibonacci(1 * time.Second)
	b = retry.WithCappedDuration(3*time.Second, b)

	if err := retry.Do(ctx, b, func(_ context.Context) error {
		// TODO: logic here
		return nil
	}); err != nil {
		// handle error
	}
}

func TestWithMaxDuration(t *testing.T) {
	t.Parallel()

	b := retry.WithMaxDuration(250*time.Millisecond, retry.BackoffFunc(func(err error) (time.Duration, error) {
		return 1 * time.Second, err
	}))

	// Take once, within timeout.
	delay, _ := b.Next(nil)
	if retry.IsStopped(delay) {
		t.Error("should not stop")
	}

	if delay > 250*time.Millisecond {
		t.Errorf("expected %v to be less than %v", delay, 250*time.Millisecond)
	}

	time.Sleep(200 * time.Millisecond)

	// Take again, remainder contines
	delay, _ = b.Next(nil)
	if retry.IsStopped(delay) {
		t.Error("should not stop")
	}

	if delay > 50*time.Millisecond {
		t.Errorf("expected %v to be less than %v", delay, 50*time.Millisecond)
	}

	time.Sleep(50 * time.Millisecond)

	// Now we stop
	delay, _ = b.Next(nil)
	if delay >= 0 {
		t.Errorf("should stop")
	}
}

func ExampleWithMaxDuration() {
	ctx := context.Background()

	b := retry.NewFibonacci(1 * time.Second)
	b = retry.WithMaxDuration(5*time.Second, b)

	if err := retry.Do(ctx, b, func(_ context.Context) error {
		// TODO: logic here
		return nil
	}); err != nil {
		// handle error
	}
}

type httpRetryableError struct {
	err  error
	resp http.Response
}

func (e *httpRetryableError) Unwrap() error {
	return e.err
}

func (e *httpRetryableError) Error() string {
	return e.err.Error()
}

func TestWithCustom(t *testing.T) {
	WithHTTPResponse := func(next retry.Backoff) retry.Backoff {
		return retry.BackoffFunc(func(err error) (time.Duration, error) {
			var herr *httpRetryableError
			if !errors.As(err, &herr) {
				return retry.Stop, err
			}
			err = herr.Unwrap()

			delay, err := next.Next(err)
			if retry.IsStopped(delay) {
				return retry.Stop, err
			}

			switch herr.resp.StatusCode {
			case 427:
				retryAfter, err := strconv.Atoi(herr.resp.Header.Get("Retry-After"))
				if err != nil {
					retryAfter = 10
				}
				delay = time.Duration(retryAfter) * time.Second
			case 500:
				delay = 2 * time.Second
			}

			// return backoff calculated by other wrappers
			return delay, err
		})
	}

	ctx := context.Background()

	b := retry.NewExponential(1 * time.Second)
	b = WithHTTPResponse(b)

	reqCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reqCount == 0 {
			w.Header().Add("Retry-After", "1")
			w.WriteHeader(427)
		} else if reqCount == 1 {
			w.WriteHeader(500)
		} else {
			fmt.Fprintln(w, "hello")
		}
		reqCount++
	}))
	defer ts.Close()

	var body []byte

	err := retry.Do(ctx, b, func(_ context.Context) error {
		resp, err := http.Get(ts.URL)

		if err == nil {
			if resp.StatusCode != 200 {
				return &httpRetryableError{
					err:  err,
					resp: *resp,
				}
			}

			defer func() {
				if err := resp.Body.Close(); err != nil {
					panic(err)
				}
			}()
			body, err = ioutil.ReadAll(resp.Body)
		}

		return err
	})

	if err != nil {
		t.Errorf("expected no error")
	}
	if len(body) == 0 {
		t.Errorf("expected non empty body")
	}
}
