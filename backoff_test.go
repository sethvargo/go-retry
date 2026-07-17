package retry_test

import (
	"context"
	"testing"
	"time"

	"github.com/sethvargo/go-retry"
)

func ExampleBackoffFunc() {
	ctx := context.Background()

	// Example backoff middleware that adds the provided duration t to the result.
	withShift := func(t time.Duration, next retry.Backoff) retry.BackoffFunc {
		return func() (time.Duration, bool) {
			val, stop := next.Next()
			if stop {
				return 0, true
			}
			return val + t, false
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

	for range 100_000 {
		b := retry.WithJitter(250*time.Millisecond, retry.BackoffFunc(func() (time.Duration, bool) {
			return 1 * time.Second, false
		}))
		val, stop := b.Next()
		if stop {
			t.Errorf("should not stop")
		}

		if min, max := 750*time.Millisecond, 1250*time.Millisecond; val < min || val > max {
			t.Errorf("expected %v to be between %v and %v", val, min, max)
		}
	}
}

func TestWithJitter_NonPositive(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		j        time.Duration
		nextVal  time.Duration
		nextStop bool
		wantVal  time.Duration
		wantStop bool
	}{
		{
			name:    "zero",
			j:       0,
			nextVal: 1 * time.Second,
			wantVal: 1 * time.Second,
		},
		{
			name:    "negative",
			j:       -5 * time.Second,
			nextVal: 1 * time.Second,
			wantVal: 1 * time.Second,
		},
		{
			name:     "stop_propagates",
			j:        0,
			nextStop: true,
			wantStop: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// A non-positive jitter must not panic (rand.Int64N panics on a
			// non-positive argument) and must return the underlying value
			// unchanged.
			b := retry.WithJitter(tc.j, retry.BackoffFunc(func() (time.Duration, bool) {
				return tc.nextVal, tc.nextStop
			}))

			val, stop := b.Next()
			if stop != tc.wantStop {
				t.Errorf("expected stop to be %t", tc.wantStop)
			}
			if val != tc.wantVal {
				t.Errorf("expected %v to be %v", val, tc.wantVal)
			}
		})
	}
}

func ExampleWithJitter() {
	ctx := context.Background()

	b := retry.NewFibonacci(1 * time.Second)
	b = retry.WithJitter(1*time.Second, b)

	if err := retry.Do(ctx, b, func(_ context.Context) error {
		// TODO: logic here
		return nil
	}); err != nil {
		// handle error
	}
}

func TestWithJitterPercent(t *testing.T) {
	t.Parallel()

	for range 100_000 {
		b := retry.WithJitterPercent(5, retry.BackoffFunc(func() (time.Duration, bool) {
			return 1 * time.Second, false
		}))
		val, stop := b.Next()
		if stop {
			t.Errorf("should not stop")
		}

		if min, max := 950*time.Millisecond, 1050*time.Millisecond; val < min || val > max {
			t.Errorf("expected %v to be between %v and %v", val, min, max)
		}
	}
}

func TestWithJitterPercent_Zero(t *testing.T) {
	t.Parallel()

	// j is a uint64, so zero is the only non-positive value it can hold.
	cases := []struct {
		name     string
		j        uint64
		nextVal  time.Duration
		nextStop bool
		wantVal  time.Duration
		wantStop bool
	}{
		{
			name:    "zero",
			j:       0,
			nextVal: 1 * time.Second,
			wantVal: 1 * time.Second,
		},
		{
			name:     "stop_propagates",
			j:        0,
			nextStop: true,
			wantStop: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// A zero jitter must not panic (rand.Int64N panics on a non-positive
			// argument) and must return the underlying value unchanged.
			b := retry.WithJitterPercent(tc.j, retry.BackoffFunc(func() (time.Duration, bool) {
				return tc.nextVal, tc.nextStop
			}))

			val, stop := b.Next()
			if stop != tc.wantStop {
				t.Errorf("expected stop to be %t", tc.wantStop)
			}
			if val != tc.wantVal {
				t.Errorf("expected %v to be %v", val, tc.wantVal)
			}
		})
	}
}

func ExampleWithJitterPercent() {
	ctx := context.Background()

	b := retry.NewFibonacci(1 * time.Second)
	b = retry.WithJitterPercent(5, b)

	if err := retry.Do(ctx, b, func(_ context.Context) error {
		// TODO: logic here
		return nil
	}); err != nil {
		// handle error
	}
}

func TestWithFullJitter(t *testing.T) {
	t.Parallel()

	for range 100_000 {
		b := retry.WithFullJitter(retry.BackoffFunc(func() (time.Duration, bool) {
			return 1 * time.Second, false
		}))
		val, stop := b.Next()
		if stop {
			t.Errorf("should not stop")
		}

		if min, max := time.Duration(0), 1*time.Second; val < min || val >= max {
			t.Errorf("expected %v to be in [%v, %v)", val, min, max)
		}
	}
}

func TestWithFullJitter_NonPositive(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		nextVal  time.Duration
		nextStop bool
		wantVal  time.Duration
		wantStop bool
	}{
		{
			name:    "zero",
			nextVal: 0,
			wantVal: 0,
		},
		{
			name:    "negative",
			nextVal: -5 * time.Second,
			wantVal: -5 * time.Second,
		},
		{
			name:     "stop_propagates",
			nextStop: true,
			wantStop: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// A non-positive base must not panic (rand.Int64N panics on a
			// non-positive argument) and must return the value unchanged.
			b := retry.WithFullJitter(retry.BackoffFunc(func() (time.Duration, bool) {
				return tc.nextVal, tc.nextStop
			}))

			val, stop := b.Next()
			if stop != tc.wantStop {
				t.Errorf("expected stop to be %t", tc.wantStop)
			}
			if val != tc.wantVal {
				t.Errorf("expected %v to be %v", val, tc.wantVal)
			}
		})
	}
}

func ExampleWithFullJitter() {
	ctx := context.Background()

	b := retry.NewFibonacci(1 * time.Second)
	b = retry.WithFullJitter(b)

	if err := retry.Do(ctx, b, func(_ context.Context) error {
		// TODO: logic here
		return nil
	}); err != nil {
		// handle error
	}
}

func TestWithMaxRetries(t *testing.T) {
	t.Parallel()

	b := retry.WithMaxRetries(3, retry.BackoffFunc(func() (time.Duration, bool) {
		return 1 * time.Second, false
	}))

	// First 3 attempts succeed
	for range 3 {
		val, stop := b.Next()
		if stop {
			t.Errorf("should not stop")
		}
		if val != 1*time.Second {
			t.Errorf("expected %v to be %v", val, 1*time.Second)
		}
	}

	// Now we stop
	val, stop := b.Next()
	if !stop {
		t.Errorf("should stop")
	}
	if val != 0 {
		t.Errorf("expected %v to be %v", val, 0)
	}
}

func TestWithMaxRetries_Zero(t *testing.T) {
	t.Parallel()

	// This test verifies the behavior of WithMaxRetries when the maximum number
	// of retries is set to 0.
	b := retry.WithMaxRetries(0, retry.BackoffFunc(func() (time.Duration, bool) {
		return 1 * time.Second, false
	}))

	val, stop := b.Next()
	if !stop {
		t.Errorf("should stop")
	}
	if val != 0 {
		t.Errorf("expected %v to be %v", val, 0)
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

	b := retry.WithCappedDuration(3*time.Second, retry.BackoffFunc(func() (time.Duration, bool) {
		return 5 * time.Second, false
	}))

	val, stop := b.Next()
	if stop {
		t.Errorf("should not stop")
	}
	if val != 3*time.Second {
		t.Errorf("expected %v to be %v", val, 3*time.Second)
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

	b := retry.WithMaxDuration(250*time.Millisecond, retry.BackoffFunc(func() (time.Duration, bool) {
		return 1 * time.Second, false
	}))

	// Take once, within timeout.
	val, stop := b.Next()
	if stop {
		t.Error("should not stop")
	}

	if val > 250*time.Millisecond {
		t.Errorf("expected %v to be less than %v", val, 250*time.Millisecond)
	}

	time.Sleep(200 * time.Millisecond)

	// Take again, remainder contines
	val, stop = b.Next()
	if stop {
		t.Error("should not stop")
	}

	if val > 50*time.Millisecond {
		t.Errorf("expected %v to be less than %v", val, 50*time.Millisecond)
	}

	time.Sleep(50 * time.Millisecond)

	// Now we stop
	val, stop = b.Next()
	if !stop {
		t.Errorf("should stop")
	}
	if val != 0 {
		t.Errorf("expected %v to be %v", val, 0)
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
