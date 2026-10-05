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

func TestFibonacciBackoff_Next(t *testing.T) {
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
				20 * time.Millisecond,
				30 * time.Millisecond,
				50 * time.Millisecond,
				80 * time.Millisecond,
			},
		},
		{
			name:  "many",
			base:  1 * time.Nanosecond,
			tries: 14,
			exp: []time.Duration{
				1 * time.Nanosecond,
				2 * time.Nanosecond,
				3 * time.Nanosecond,
				5 * time.Nanosecond,
				8 * time.Nanosecond,
				13 * time.Nanosecond,
				21 * time.Nanosecond,
				34 * time.Nanosecond,
				55 * time.Nanosecond,
				89 * time.Nanosecond,
				144 * time.Nanosecond,
				233 * time.Nanosecond,
				377 * time.Nanosecond,
				610 * time.Nanosecond,
			},
		},
		{
			name:  "overflow",
			base:  100_000 * time.Hour,
			tries: 10,
			exp: []time.Duration{
				100_000 * time.Hour,
				200_000 * time.Hour,
				300_000 * time.Hour,
				500_000 * time.Hour,
				800_000 * time.Hour,
				1_300_000 * time.Hour,
				2_100_000 * time.Hour,
				math.MaxInt64,
				math.MaxInt64,
				math.MaxInt64,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := retry.NewFibonacci(tc.base)

			resultsCh := make(chan time.Duration, tc.tries)
			for range tc.tries {
				go func() {
					r, _ := b.Next()
					resultsCh <- r
				}()
			}

			results := make([]time.Duration, tc.tries)
			for i := range tc.tries {
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

func TestNewFibonacci(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		base      time.Duration
		wantPanic bool
	}{
		{
			name:      "panics_on_zero",
			base:      0,
			wantPanic: true,
		},
		{
			name:      "panics_on_negative",
			base:      -1 * time.Second,
			wantPanic: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.wantPanic {
				defer func() {
					if recover() == nil {
						t.Errorf("expected panic")
					}
				}()
			}
			retry.NewFibonacci(tc.base)
		})
	}
}

func TestFibonacci(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		base      time.Duration
		failUntil int
	}{
		{
			name:      "retries_until_success",
			base:      1 * time.Nanosecond,
			failUntil: 3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			calls := 0
			err := retry.Fibonacci(context.Background(), tc.base, func(_ context.Context) error {
				calls++
				if calls < tc.failUntil {
					return retry.RetryableError(errors.New("retry"))
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if calls != tc.failUntil {
				t.Errorf("expected %d to be %d", calls, tc.failUntil)
			}
		})
	}
}

func ExampleNewFibonacci() {
	b := retry.NewFibonacci(1 * time.Second)

	for range 5 {
		val, _ := b.Next()
		fmt.Printf("%v\n", val)
	}
	// Output:
	// 1s
	// 2s
	// 3s
	// 5s
	// 8s
}
