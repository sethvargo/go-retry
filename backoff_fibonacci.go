package retry

import (
	"context"
	"sync/atomic"
	"time"
	"unsafe"
)

type state [2]time.Duration

type fibonacciBackoff struct {
	state unsafe.Pointer
}

// Fibonacci is a wrapper around Retry that uses a Fibonacci backoff. It panics
// if the given base is less than zero.
func Fibonacci(ctx context.Context, base time.Duration, f RetryFunc) error {
	return Do(ctx, NewFibonacci(base), f)
}

// NewFibonacci creates a new Fibonacci backoff using the starting value of
// base. The wait time is the sum of the previous two wait times on each failed
// attempt (1, 1, 2, 3, 5, 8, 13...). It panics if the given base is less than
// zero.
func NewFibonacci(base time.Duration) Backoff {
	if base <= 0 {
		panic("base must be greater than 0")
	}

	return &fibonacciBackoff{
		state: unsafe.Pointer(&state{0, base}),
	}
}

// Next implements Backoff. It is safe for concurrent use.
func (b *fibonacciBackoff) Next() (time.Duration, bool) {
	for {
		curr := atomic.LoadPointer(&b.state)
		currState := (*state)(curr)
		next := currState[0] + currState[1]

		if atomic.CompareAndSwapPointer(&b.state, curr, unsafe.Pointer(&state{currState[1], next})) {
			return next, false
		}
	}
}
