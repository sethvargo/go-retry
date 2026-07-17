// The retry-loop tests below use the testing/synctest package (GA in Go 1.25)
// to drive the loop's inter-attempt sleeps against a fake clock, making them
// deterministic and instant instead of relying on real time.
// Background and rationale: https://go.dev/blog/testing-time

package retry_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"testing"
	"testing/synctest"
	"time"

	"github.com/sethvargo/go-retry"
)

func TestExponential(t *testing.T) {
	t.Parallel()

	// The retry loop sleeps between attempts, so run it in a synctest bubble
	// (see the file header) rather than dodging real time with a tiny base.
	synctest.Test(t, func(t *testing.T) {
		var calls int
		err := retry.Exponential(t.Context(), 1*time.Second, func(_ context.Context) error {
			calls++
			if calls < 3 {
				return retry.RetryableError(errors.New("again"))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if calls != 3 {
			t.Errorf("expected 3 calls, got %d", calls)
		}
	})
}

func TestNewExponential_panics(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		base time.Duration
	}{
		{"zero", 0},
		{"negative", -1},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			defer func() {
				if r := recover(); r == nil {
					t.Error("expected panic, got none")
				}
			}()
			retry.NewExponential(tc.base)
		})
	}
}

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
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := retry.NewExponential(tc.base)

			resultsCh := make(chan time.Duration, tc.tries)
			for i := 0; i < tc.tries; i++ {
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
			sort.Slice(results, func(i, j int) bool {
				return results[i] < results[j]
			})

			if !reflect.DeepEqual(results, tc.exp) {
				t.Errorf("expected \n\n%v\n\n to be \n\n%v\n\n", results, tc.exp)
			}
		})
	}
}

func ExampleNewExponential() {
	b := retry.NewExponential(1 * time.Second)

	for i := 0; i < 5; i++ {
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
