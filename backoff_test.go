package retry_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/sethvargo/go-retry"
)

func ExampleBackoff() {
	// A custom backoff: wait 100ms more on each retry, and stop after 3 retries.
	linear := retry.Backoff(func(attempt uint64) (time.Duration, bool) {
		if attempt >= 3 {
			return 0, true
		}
		return time.Duration(attempt+1) * 100 * time.Millisecond, false
	})

	// A custom middleware: add shift to every wait from next.
	withShift := func(shift time.Duration, next retry.Backoff) retry.Backoff {
		return func(attempt uint64) (time.Duration, bool) {
			val, stop := next(attempt)
			if stop {
				return 0, true
			}
			return val + shift, false
		}
	}

	b := withShift(1*time.Second, linear)
	for attempt := range uint64(4) {
		fmt.Println(b(attempt))
	}
	// Output:
	// 1.1s false
	// 1.2s false
	// 1.3s false
	// 0s true
}

func TestBackoff(t *testing.T) {
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

			b := retry.Backoff(func(uint64) (time.Duration, bool) {
				return tc.val, tc.stop
			})

			val, stop := b(0)
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
			name: "zero_with_negative_backoff",
			j:    0,
			val:  -5 * time.Second,
			min:  0,
			max:  0,
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

			b := retry.WithJitter(tc.j, retry.Backoff(func(uint64) (time.Duration, bool) {
				return tc.val, tc.stop
			}))

			for range iters {
				val, stop := b(0)
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
	// Each wait is 2s +/- 500ms.
	b := retry.WithJitter(500*time.Millisecond, retry.NewConstant(2*time.Second))

	for attempt := range uint64(3) {
		next, _ := b(attempt)
		fmt.Println(next >= 1500*time.Millisecond && next <= 2500*time.Millisecond)
	}
	// Output:
	// true
	// true
	// true
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
			name: "zero_with_negative_backoff",
			j:    0,
			val:  -5 * time.Second,
			min:  0,
			max:  0,
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
			min:   math.MaxInt64 / 100 * 98, // 1% below, plus room for float64 rounding
			max:   math.MaxInt64,
		},
		{
			name:  "maximum_percentage",
			j:     100,
			val:   math.MaxInt64,
			iters: 1_000,
			min:   0,
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
			min:   0,
			max:   2 * time.Second,
		},
		{
			name:  "above_hundred_double",
			j:     200,
			val:   1 * time.Second,
			iters: 10_000,
			min:   0,
			max:   2 * time.Second,
		},
		{
			name:  "above_hundred_overflow",
			j:     1 << 62,
			val:   1 * time.Second,
			iters: 10_000,
			min:   0,
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

			b := retry.WithJitterPercent(tc.j, retry.Backoff(func(uint64) (time.Duration, bool) {
				return tc.val, tc.stop
			}))

			for range iters {
				val, stop := b(0)
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
	// Each wait is 2s +/- 10%.
	b := retry.WithJitterPercent(10, retry.NewConstant(2*time.Second))

	for attempt := range uint64(3) {
		next, _ := b(attempt)
		fmt.Println(next >= 1800*time.Millisecond && next <= 2200*time.Millisecond)
	}
	// Output:
	// true
	// true
	// true
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
			min:  0,
			max:  0,
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

			b := retry.WithFullJitter(retry.Backoff(func(uint64) (time.Duration, bool) {
				return tc.val, tc.stop
			}))

			for range iters {
				val, stop := b(0)
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
	// Each wait is anywhere in [0, 2s).
	b := retry.WithFullJitter(retry.NewConstant(2 * time.Second))

	for attempt := range uint64(3) {
		next, _ := b(attempt)
		fmt.Println(next >= 0 && next < 2*time.Second)
	}
	// Output:
	// true
	// true
	// true
}

func TestWithMaxRetries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		max           uint64
		val           time.Duration
		innerStop     bool
		wantSuccesses int
		want          time.Duration
	}{
		{
			name:          "exhausts_after_max",
			max:           3,
			val:           1 * time.Second,
			wantSuccesses: 3,
			want:          1 * time.Second,
		},
		{
			name:          "zero",
			max:           0,
			wantSuccesses: 0,
		},
		{
			name:          "stop_propagates",
			max:           3,
			innerStop:     true,
			wantSuccesses: 0,
		},
		{
			name:          "negative_backoff",
			max:           1,
			val:           -5 * time.Second,
			wantSuccesses: 1,
			want:          0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := retry.WithMaxRetries(tc.max, retry.Backoff(func(uint64) (time.Duration, bool) {
				return tc.val, tc.innerStop
			}))

			for i := range tc.wantSuccesses {
				val, stop := b(uint64(i))
				if stop {
					t.Fatalf("should not stop on attempt %d", i)
				}
				if val != tc.want {
					t.Errorf("expected %v to be %v", val, tc.want)
				}
			}

			val, stop := b(uint64(tc.wantSuccesses))
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
	b := retry.WithMaxRetries(3, retry.NewFibonacci(1*time.Second))

	for attempt := range uint64(5) {
		fmt.Println(b(attempt))
	}
	// Output:
	// 1s false
	// 1s false
	// 2s false
	// 0s true
	// 0s true
}

func TestWithCappedDuration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		cap      time.Duration
		backoff  retry.Backoff
		want     time.Duration
		wantStop bool
	}{
		{
			name:    "caps_when_exceeded",
			cap:     3 * time.Second,
			backoff: retry.Backoff(func(uint64) (time.Duration, bool) { return 5 * time.Second, false }),
			want:    3 * time.Second,
		},
		{
			name:    "zero",
			cap:     5 * time.Second,
			backoff: retry.Backoff(func(uint64) (time.Duration, bool) { return 0, false }),
			want:    0,
		},
		{
			name:    "full_jitter",
			cap:     5 * time.Second,
			backoff: retry.WithFullJitter(retry.Backoff(func(uint64) (time.Duration, bool) { return 1 * time.Nanosecond, false })),
			want:    0,
		},
		{
			name:    "negative_cap",
			cap:     -1 * time.Second,
			backoff: retry.Backoff(func(uint64) (time.Duration, bool) { return 1 * time.Second, false }),
			want:    0,
		},
		{
			name:    "negative_backoff",
			cap:     5 * time.Second,
			backoff: retry.Backoff(func(uint64) (time.Duration, bool) { return -5 * time.Second, false }),
			want:    0,
		},
		{
			name:     "stop_propagates",
			cap:      5 * time.Second,
			backoff:  retry.Backoff(func(uint64) (time.Duration, bool) { return 1 * time.Second, true }),
			want:     0,
			wantStop: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := retry.WithCappedDuration(tc.cap, tc.backoff)
			val, stop := b(0)
			if stop != tc.wantStop {
				t.Fatalf("expected stop to be %t", tc.wantStop)
			}
			if val != tc.want {
				t.Errorf("expected %v to be %v", val, tc.want)
			}
		})
	}
}

func ExampleWithCappedDuration() {
	b := retry.WithCappedDuration(5*time.Second, retry.NewExponential(1*time.Second))

	for attempt := range uint64(5) {
		fmt.Println(b(attempt))
	}
	// Output:
	// 1s false
	// 2s false
	// 4s false
	// 5s false
	// 5s false
}
