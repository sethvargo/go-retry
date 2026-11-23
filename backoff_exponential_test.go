package retry_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/sethvargo/go-retry"
)

func TestExponentialBackoff_Next(t *testing.T) {
	t.Parallel()

	overflowBase := 100_000 * time.Hour
	saturatedExp := make([]time.Duration, 0, 100)
	for next := overflowBase; next > 0; next <<= 1 {
		saturatedExp = append(saturatedExp, next)
	}
	for len(saturatedExp) < 100 {
		saturatedExp = append(saturatedExp, math.MaxInt64)
	}

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
		{
			name:  "stays_saturated",
			base:  overflowBase,
			tries: 100,
			exp:   saturatedExp,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := retry.NewExponential(tc.base)

			results := make([]time.Duration, tc.tries)
			for i := range tc.tries {
				results[i], _ = b(uint64(i))
			}

			if !reflect.DeepEqual(results, tc.exp) {
				t.Errorf("expected \n\n%v\n\n to be \n\n%v\n\n", results, tc.exp)
			}
		})
	}
}

func ExampleNewExponential() {
	b := retry.NewExponential(1 * time.Second)

	for i := range 5 {
		val, _ := b(uint64(i))
		fmt.Printf("%v\n", val)
	}
	// Output:
	// 1s
	// 2s
	// 4s
	// 8s
	// 16s
}

func TestExponential(t *testing.T) {
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
			err := retry.Exponential(context.Background(), tc.base, func(_ context.Context) error {
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

func TestNewExponential(t *testing.T) {
	t.Parallel()

	t.Run("panics_on_zero", func(t *testing.T) {
		t.Parallel()

		defer func() {
			if recover() == nil {
				t.Errorf("expected panic")
			}
		}()
		retry.NewExponential(0)
	})

	t.Run("panics_on_negative", func(t *testing.T) {
		t.Parallel()

		defer func() {
			if recover() == nil {
				t.Errorf("expected panic")
			}
		}()
		retry.NewExponential(-1 * time.Second)
	})
}
