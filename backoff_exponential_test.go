package retry

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestExponentialBackoff(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		base  time.Duration
		tries int
		exp   []time.Duration
		err   bool
	}{
		{
			name: "zero",
			err:  true,
		},
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
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b, err := NewExponential(tc.base)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}
			if b == nil {
				return
			}

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
	b, err := NewExponential(1 * time.Second)
	if err != nil {
		// handle err
	}

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
