package retry_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/sethvargo/go-retry"
)

func TestExponentialBackoff(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		base  time.Duration
		tries int
		exp   []time.Duration
	}{
		{
			name:  "single",
			base:  1 * time.Nanosecond,
			tries: 1,
			exp: []time.Duration{
				1 * time.Nanosecond,
			},
		},
		{
			name:  "many",
			base:  1 * time.Nanosecond,
			tries: 14,
			exp: []time.Duration{
				1 * time.Nanosecond,
				2 * time.Nanosecond,
				4 * time.Nanosecond,
				8 * time.Nanosecond,
				16 * time.Nanosecond,
				32 * time.Nanosecond,
				64 * time.Nanosecond,
				128 * time.Nanosecond,
				256 * time.Nanosecond,
				512 * time.Nanosecond,
				1024 * time.Nanosecond,
				2048 * time.Nanosecond,
				4096 * time.Nanosecond,
				8192 * time.Nanosecond,
			},
		},
		{
			name:  "overflow",
			base:  100_000 * time.Hour,
			tries: 10,
			exp: []time.Duration{
				100_000 * time.Hour,
				200_000 * time.Hour,
				400_000 * time.Hour,
				800_000 * time.Hour,
				1_600_000 * time.Hour,
				math.MaxInt64,
				math.MaxInt64,
				math.MaxInt64,
				math.MaxInt64,
				math.MaxInt64,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := retry.NewExponential(tc.base)

			resultsCh := make(chan time.Duration, tc.tries)
			for range tc.tries {
				go func() {
					r, _ := b.Next()
					resultsCh <- r
				}()
			}

			results := make([]time.Duration, tc.tries)
			for i := 0; i < tc.tries; i++ {
				select {
				case val := <-resultsCh:
					results[i] = val
				case <-time.After(5 * time.Second):
					t.Fatal("timeout")
				}
			}
			slices.Sort(results)

			if !reflect.DeepEqual(results, tc.exp) {
				t.Errorf("expected \n\n%v\n\n to be \n\n%v\n\n", results, tc.exp)
			}
		})
	}
}

func ExampleNewExponential() {
	b := retry.NewExponential(1 * time.Second)

	for range 5 {
		val, _ := b.Next()
		fmt.Printf("%v\n", val)
	}
	// Output:
	// 1s
	// 2s
	// 4s
	// 8s
	// 16s
}

func TestExponentialBackoff_ConcurrentOverflow(t *testing.T) {
	t.Parallel()

	// Many concurrent Next calls on an overflow-prone base must never observe a
	// value outside the doubling sequence or MaxInt64. A racy attempt counter
	// lets the shift run past the overflow point and wrap to a bogus positive.
	const tries = 100
	base := 100_000 * time.Hour

	b := retry.NewExponential(base)

	resultsCh := make(chan time.Duration, tries)
	for range tries {
		go func() {
			r, _ := b.Next()
			resultsCh <- r
		}()
	}

	results := make([]time.Duration, tries)
	for i := range tries {
		select {
		case val := <-resultsCh:
			results[i] = val
		case <-time.After(5 * time.Second):
			t.Fatal("timeout")
		}
	}
	slices.Sort(results)

	want := make([]time.Duration, 0, tries)
	for next := base; next > 0; next <<= 1 {
		want = append(want, next)
	}
	for len(want) < tries {
		want = append(want, math.MaxInt64)
	}
	slices.Sort(want)

	if !reflect.DeepEqual(results, want) {
		t.Errorf("expected \n\n%v\n\n to be \n\n%v\n\n", results, want)
	}
}

func TestExponential(t *testing.T) {
	t.Parallel()

	calls := 0
	if err := retry.Exponential(context.Background(), 1*time.Nanosecond, func(_ context.Context) error {
		calls++
		if calls < 3 {
			return retry.RetryableError(errors.New("retry"))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Errorf("expected %d to be %d", calls, 3)
	}
}

func TestNewExponential_panics(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		base time.Duration
	}{
		{name: "zero", base: 0},
		{name: "negative", base: -1 * time.Second},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			defer func() {
				if recover() == nil {
					t.Errorf("expected panic")
				}
			}()
			retry.NewExponential(tc.base)
		})
	}
}
