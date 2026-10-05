package retry_test

import (
	"context"
	"math"
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

	// Middleware wraps another backoff:
	b := retry.NewFibonacci(1 * time.Second)
	b = withShift(5*time.Second, b)

	if err := retry.Do(ctx, b, func(ctx context.Context) error {
		// Actual retry logic here
		return nil
	}); err != nil {
		// handle error
	}
}

func TestBackoffFunc_Next(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		val  time.Duration
		stop bool
	}{
		{
			name: "returns_value",
			val:  1 * time.Second,
			stop: false,
		},
		{
			name: "propagates_stop",
			val:  0,
			stop: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := retry.BackoffFunc(func() (time.Duration, bool) {
				return tc.val, tc.stop
			})

			val, stop := b.Next()
			if val != tc.val {
				t.Errorf("expected %v to be %v", val, tc.val)
			}
			if stop != tc.stop {
				t.Errorf("expected stop to be %t", tc.stop)
			}
		})
	}
}

func TestWithJitter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		j        time.Duration
		val      time.Duration
		stop     bool
		iters    int
		min      time.Duration
		max      time.Duration
		wantStop bool
	}{
		{
			name:  "within_range",
			j:     250 * time.Millisecond,
			val:   1 * time.Second,
			iters: 100_000,
			min:   750 * time.Millisecond,
			max:   1250 * time.Millisecond,
		},
		{
			name: "zero",
			j:    0,
			val:  1 * time.Second,
			min:  1 * time.Second,
			max:  1 * time.Second,
		},
		{
			name: "negative",
			j:    -5 * time.Second,
			val:  1 * time.Second,
			min:  1 * time.Second,
			max:  1 * time.Second,
		},
		{
			name:     "stop_propagates",
			j:        0,
			stop:     true,
			wantStop: true,
		},
		{
			name:  "saturated_backoff",
			j:     2,
			val:   math.MaxInt64,
			iters: 1_000,
			min:   math.MaxInt64 - 2,
			max:   math.MaxInt64,
		},
		{
			name:  "largest_signed_random_bound",
			j:     math.MaxInt64 / 2,
			val:   math.MaxInt64,
			iters: 1_000,
			min:   math.MaxInt64 - math.MaxInt64/2,
			max:   math.MaxInt64,
		},
		{
			name:  "random_bound_overflow",
			j:     1 << 62,
			val:   math.MaxInt64,
			iters: 1_000,
			min:   math.MaxInt64 - (1 << 62),
			max:   math.MaxInt64,
		},
		{
			name:  "maximum_jitter",
			j:     math.MaxInt64,
			val:   0,
			iters: 1_000,
			min:   0,
			max:   math.MaxInt64 - 1,
		},
		{
			name:  "maximum_jitter_and_backoff",
			j:     math.MaxInt64,
			val:   math.MaxInt64,
			iters: 1_000,
			min:   0,
			max:   math.MaxInt64,
		},
		{
			name:  "negative_backoff",
			j:     math.MaxInt64,
			val:   math.MinInt64,
			iters: 1_000,
			min:   0,
			max:   0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			iters := tc.iters
			if iters == 0 {
				iters = 1
			}

			b := retry.WithJitter(tc.j, retry.BackoffFunc(func() (time.Duration, bool) {
				return tc.val, tc.stop
			}))

			for range iters {
				val, stop := b.Next()
				if stop != tc.wantStop {
					t.Fatalf("expected stop to be %t", tc.wantStop)
				}
				if stop {
					continue
				}
				if val < tc.min || val > tc.max {
					t.Fatalf("expected %v to be between %v and %v", val, tc.min, tc.max)
				}
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

	cases := []struct {
		name     string
		j        uint64
		val      time.Duration
		stop     bool
		iters    int
		min      time.Duration
		max      time.Duration
		wantStop bool
	}{
		{
			name:  "within_range",
			j:     5,
			val:   1 * time.Second,
			iters: 100_000,
			min:   950 * time.Millisecond,
			max:   1050 * time.Millisecond,
		},
		{
			name: "zero",
			j:    0,
			val:  1 * time.Second,
			min:  1 * time.Second,
			max:  1 * time.Second,
		},
		{
			name:     "stop_propagates",
			j:        0,
			stop:     true,
			wantStop: true,
		},
		{
			name:  "saturated_backoff",
			j:     1,
			val:   math.MaxInt64,
			iters: 1_000,
			min:   math.MaxInt64,
			max:   math.MaxInt64,
		},
		{
			name:  "maximum_percentage",
			j:     100,
			val:   math.MaxInt64,
			iters: 1_000,
			min:   math.MaxInt64 / 100,
			max:   math.MaxInt64,
		},
		{
			name:  "negative_backoff",
			j:     100,
			val:   math.MinInt64,
			iters: 1_000,
			min:   0,
			max:   0,
		},
		{
			name:  "above_hundred_just_above",
			j:     101,
			val:   1 * time.Second,
			iters: 10_000,
			min:   1,
			max:   2 * time.Second,
		},
		{
			name:  "above_hundred_double",
			j:     200,
			val:   1 * time.Second,
			iters: 10_000,
			min:   1,
			max:   2 * time.Second,
		},
		{
			name:  "above_hundred_overflow",
			j:     1 << 62,
			val:   1 * time.Second,
			iters: 10_000,
			min:   1,
			max:   2 * time.Second,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			iters := tc.iters
			if iters == 0 {
				iters = 1
			}

			b := retry.WithJitterPercent(tc.j, retry.BackoffFunc(func() (time.Duration, bool) {
				return tc.val, tc.stop
			}))

			for range iters {
				val, stop := b.Next()
				if stop != tc.wantStop {
					t.Fatalf("expected stop to be %t", tc.wantStop)
				}
				if stop {
					continue
				}
				if val < tc.min || val > tc.max {
					t.Fatalf("expected %v to be between %v and %v", val, tc.min, tc.max)
				}
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

	cases := []struct {
		name     string
		val      time.Duration
		stop     bool
		iters    int
		min      time.Duration
		max      time.Duration
		wantStop bool
	}{
		{
			name:  "within_range",
			val:   1 * time.Second,
			iters: 100_000,
			min:   0,
			max:   1*time.Second - 1,
		},
		{
			name: "zero",
			val:  0,
			min:  0,
			max:  0,
		},
		{
			name: "negative",
			val:  -5 * time.Second,
			min:  -5 * time.Second,
			max:  -5 * time.Second,
		},
		{
			name:     "stop_propagates",
			stop:     true,
			wantStop: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			iters := tc.iters
			if iters == 0 {
				iters = 1
			}

			b := retry.WithFullJitter(retry.BackoffFunc(func() (time.Duration, bool) {
				return tc.val, tc.stop
			}))

			for range iters {
				val, stop := b.Next()
				if stop != tc.wantStop {
					t.Fatalf("expected stop to be %t", tc.wantStop)
				}
				if stop {
					continue
				}
				if val < tc.min || val > tc.max {
					t.Fatalf("expected %v to be between %v and %v", val, tc.min, tc.max)
				}
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

	cases := []struct {
		name          string
		max           uint64
		wantSuccesses int
	}{
		{
			name:          "exhausts_after_max",
			max:           3,
			wantSuccesses: 3,
		},
		{
			name:          "zero",
			max:           0,
			wantSuccesses: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := retry.WithMaxRetries(tc.max, retry.BackoffFunc(func() (time.Duration, bool) {
				return 1 * time.Second, false
			}))

			for i := range tc.wantSuccesses {
				val, stop := b.Next()
				if stop {
					t.Fatalf("should not stop on attempt %d", i)
				}
				if val != 1*time.Second {
					t.Errorf("expected %v to be %v", val, 1*time.Second)
				}
			}

			val, stop := b.Next()
			if !stop {
				t.Errorf("should stop")
			}
			if val != 0 {
				t.Errorf("expected %v to be %v", val, time.Duration(0))
			}
		})
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

	cases := []struct {
		name    string
		cap     time.Duration
		backoff retry.Backoff
		want    time.Duration
	}{
		{
			name:    "caps_when_exceeded",
			cap:     3 * time.Second,
			backoff: retry.BackoffFunc(func() (time.Duration, bool) { return 5 * time.Second, false }),
			want:    3 * time.Second,
		},
		{
			name:    "zero",
			cap:     5 * time.Second,
			backoff: retry.BackoffFunc(func() (time.Duration, bool) { return 0, false }),
			want:    0,
		},
		{
			name:    "full_jitter",
			cap:     5 * time.Second,
			backoff: retry.WithFullJitter(retry.BackoffFunc(func() (time.Duration, bool) { return 1 * time.Nanosecond, false })),
			want:    0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := retry.WithCappedDuration(tc.cap, tc.backoff)
			val, stop := b.Next()
			if stop {
				t.Fatal("should not stop")
			}
			if val != tc.want {
				t.Errorf("expected %v to be %v", val, tc.want)
			}
		})
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

	cases := []struct {
		name    string
		timeout time.Duration
		backoff retry.Backoff
		wantMax time.Duration
	}{
		{
			name:    "caps_to_remaining",
			timeout: 250 * time.Millisecond,
			backoff: retry.BackoffFunc(func() (time.Duration, bool) { return 1 * time.Second, false }),
			wantMax: 250 * time.Millisecond,
		},
		{
			name:    "zero",
			timeout: 10 * time.Second,
			backoff: retry.BackoffFunc(func() (time.Duration, bool) { return 0, false }),
			wantMax: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := retry.WithMaxDuration(tc.timeout, tc.backoff)
			val, stop := b.Next()
			if stop {
				t.Fatal("should not stop")
			}
			if val > tc.wantMax {
				t.Errorf("expected %v to be at most %v", val, tc.wantMax)
			}
		})
	}

	t.Run("decrements_over_time", func(t *testing.T) {
		t.Parallel()

		b := retry.WithMaxDuration(250*time.Millisecond, retry.BackoffFunc(func() (time.Duration, bool) {
			return 1 * time.Second, false
		}))

		val, stop := b.Next()
		if stop {
			t.Error("should not stop")
		}
		if val > 250*time.Millisecond {
			t.Errorf("expected %v to be less than %v", val, 250*time.Millisecond)
		}

		time.Sleep(200 * time.Millisecond)

		val, stop = b.Next()
		if stop {
			t.Error("should not stop")
		}
		if val > 50*time.Millisecond {
			t.Errorf("expected %v to be less than %v", val, 50*time.Millisecond)
		}

		time.Sleep(50 * time.Millisecond)

		val, stop = b.Next()
		if !stop {
			t.Errorf("should stop")
		}
		if val != 0 {
			t.Errorf("expected %v to be %v", val, time.Duration(0))
		}
	})
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
