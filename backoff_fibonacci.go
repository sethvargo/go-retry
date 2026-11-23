package retry

import (
	"context"
	"math"
	"math/bits"
	"sync/atomic"
	"time"
)

// fibonacciNumbers holds F(1) through F(92). F(93) overflows int64, so every
// later attempt saturates.
var fibonacciNumbers = func() [92]uint64 {
	var f [92]uint64
	f[0], f[1] = 1, 1
	for i := 2; i < len(f); i++ {
		f[i] = f[i-1] + f[i-2]
	}
	return f
}()

type fibonacciBackoff struct {
	base    uint64
	attempt atomic.Uint64
}

var _ Backoff = (*fibonacciBackoff)(nil)

// Fibonacci is a wrapper around [Do] that uses a Fibonacci backoff. See
// [NewFibonacci].
func Fibonacci(ctx context.Context, base time.Duration, f RetryFunc) error {
	return Do(ctx, NewFibonacci(base), f)
}

// NewFibonacci creates a new Fibonacci backoff using the starting value of
// base. The wait time is the sum of the previous two wait times on each failed
// attempt (1, 1, 2, 3, 5, 8, 13...).
//
// Once it overflows, the function constantly returns the maximum time.Duration
// for a 64-bit integer.
//
// It panics if base is less than or equal to zero.
func NewFibonacci(base time.Duration) Backoff {
	if base <= 0 {
		panic("base must be greater than 0")
	}

	return &fibonacciBackoff{
		base: uint64(base),
	}
}

// Next implements Backoff. It is safe for concurrent use.
func (b *fibonacciBackoff) Next() (time.Duration, bool) {
	for {
		attempt := b.attempt.Load()
		if attempt >= uint64(len(fibonacciNumbers)) {
			return math.MaxInt64, false
		}

		// Saturate on overflow without advancing, so the sequence ends at the
		// maximum duration.
		hi, lo := bits.Mul64(b.base, fibonacciNumbers[attempt])
		if hi != 0 || lo > math.MaxInt64 {
			return math.MaxInt64, false
		}

		if b.attempt.CompareAndSwap(attempt, attempt+1) {
			return time.Duration(lo), false
		}
	}
}
