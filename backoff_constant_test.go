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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := retry.NewConstant(tc.base)

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

func TestConstant(t *testing.T) {
	t.Parallel()

	calls := 0
	if err := retry.Constant(context.Background(), 1*time.Nanosecond, func(_ context.Context) error {
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

func TestNewConstant_panics(t *testing.T) {
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
			retry.NewConstant(tc.base)
		})
	}
}
