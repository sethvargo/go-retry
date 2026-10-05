package retry_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/sethvargo/go-retry"
)

func TestNewConstant(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		base      time.Duration
		tries     int
		exp       []time.Duration
		wantPanic bool
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
			name:  "constant",
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
				retry.NewConstant(tc.base)
				return
			}

			b := retry.NewConstant(tc.base)

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

func TestConstant(t *testing.T) {
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
			err := retry.Constant(context.Background(), tc.base, func(_ context.Context) error {
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

func ExampleNewConstant() {
	b := retry.NewConstant(1 * time.Second)

	for range 5 {
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
