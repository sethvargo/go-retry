package retry

import (
	"context"
	"time"
)

// TODO add tests

// RepeatFunc is a function passed to retry.
type RepeatFunc func(ctx context.Context) error

// Repeat wraps a function with a backoff to repeat until it returns an error, or the backoff
// signals to stop.
// The provided context is passed to the RepeatFunc.
func Repeat(ctx context.Context, b Backoff, f RepeatFunc) error {
	for {
		// Return immediately if ctx is canceled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := f(ctx); err != nil {
			return err
		}

		next, stop := b.Next()
		if stop {
			return nil
		}

		// ctx.Done() has priority, so we test it alone first
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		t := time.NewTimer(next)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
			continue
		}
	}
}

// TODO make the above like repeat.DoUntilError and then have a repeat.Do that takes an
// error handling function and keeps going
