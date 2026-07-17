// The retry-loop tests below use the testing/synctest package (GA in Go 1.25)
// to drive the loop's inter-attempt sleeps against a fake clock, making them
// deterministic and instant instead of relying on real time.
// Background and rationale: https://go.dev/blog/testing-time

package retry_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"testing/synctest"
	"time"

	"github.com/sethvargo/go-retry"
)

func TestConstant(t *testing.T) {
	t.Parallel()

	// The retry loop sleeps between attempts, so run it in a synctest bubble
	// (see the file header) rather than dodging real time with a tiny base.
	synctest.Test(t, func(t *testing.T) {
		var calls int
		err := retry.Constant(t.Context(), 1*time.Second, func(_ context.Context) error {
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

func TestNewConstant_panics(t *testing.T) {
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
			retry.NewConstant(tc.base)
		})
	}
}

func TestConstantBackoff(t *testing.T) {
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
			name:  "max",
			base:  10 * time.Millisecond,
			tries: 5,
			exp: []time.Duration{
				10 * time.Millisecond,
				10 * time.Millisecond,
				10 * time.Millisecond,
				10 * time.Millisecond,
				10 * time.Millisecond,
			},
		},
		{
			name:  "many",
			base:  1 * time.Nanosecond,
			tries: 14,
			exp: []time.Duration{
				1 * time.Nanosecond,
				1 * time.Nanosecond,
				1 * time.Nanosecond,
				1 * time.Nanosecond,
				1 * time.Nanosecond,
				1 * time.Nanosecond,
				1 * time.Nanosecond,
				1 * time.Nanosecond,
				1 * time.Nanosecond,
				1 * time.Nanosecond,
				1 * time.Nanosecond,
				1 * time.Nanosecond,
				1 * time.Nanosecond,
				1 * time.Nanosecond,
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := retry.NewConstant(tc.base)

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

func ExampleNewConstant() {
	b := retry.NewConstant(1 * time.Second)

	for i := 0; i < 5; i++ {
		val, _ := b.Next()
		fmt.Printf("%v\n", val)
	}
	// Output:
	// 1s
	// 1s
	// 1s
	// 1s
	// 1s
}
