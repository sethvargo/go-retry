package retry

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestFibonacciBackoff(t *testing.T) {
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
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b, err := NewFibonacci(tc.base)
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

func ExampleNewFibonacci() {
	b, err := NewFibonacci(1 * time.Second)
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
	// 3s
	// 5s
	// 8s
}
