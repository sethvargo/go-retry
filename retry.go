// Package retry retries Go functions that fail temporarily, waiting between
// attempts according to a [Backoff].
//
// A Backoff is a function of the attempt number. Middleware such as
// [WithMaxRetries] and [WithJitter] wraps one Backoff in another. The function
// passed to [Do] marks the errors to retry with [RetryableError], and [Do]
// returns any other error right away.
package retry

import (
	"context"
	"errors"
	"time"
)

// RetryFunc is a function passed to [Do].
type RetryFunc func(ctx context.Context) error

// RetryFuncValue is a function passed to [DoValue] which returns a value.
type RetryFuncValue[T any] func(ctx context.Context) (T, error)

type retryableError struct {
	err error
}

// RetryableError marks err as retryable. It returns nil if err is nil.
func RetryableError(err error) error {
	if err == nil {
		return nil
	}
	return &retryableError{err}
}

// Unwrap implements error wrapping.
func (e *retryableError) Unwrap() error {
	return e.err
}

// Error returns the error string.
func (e *retryableError) Error() string {
	return "retryable: " + e.err.Error()
}

// DoValue calls f until it succeeds, returns an error not marked with
// [RetryableError], the backoff stops, or ctx is done. The provided context is
// the same context passed to f. To limit the total time spent retrying, use
// [context.WithTimeout] on ctx.
//
// When the backoff stops, DoValue returns the error that was passed to
// [RetryableError]. When ctx is done, it returns [context.Cause] joined with the
// last error from f, if there was one.
func DoValue[T any](ctx context.Context, b Backoff, f RetryFuncValue[T]) (T, error) {
	var nilT T
	var lastErr error

	// Created on the first wait, so calls that succeed on the first try never
	// allocate a timer.
	var t *time.Timer

	for attempt := uint64(0); ; attempt++ {
		if err := context.Cause(ctx); err != nil {
			return nilT, errors.Join(err, lastErr)
		}

		v, err := f(ctx)
		if err == nil {
			return v, nil
		}

		rerr, ok := errors.AsType[*retryableError](err)
		if !ok {
			return nilT, err
		}

		next, stop := b(attempt)
		if stop {
			return nilT, rerr.Unwrap()
		}
		lastErr = rerr.Unwrap()

		// If ctx ends during the wait, the check at the top of the loop returns.
		if t == nil {
			t = time.NewTimer(next)
		} else {
			t.Reset(next)
		}
		select {
		case <-ctx.Done():
			t.Stop()
		case <-t.C:
		}
	}
}

// Do is [DoValue] for a function that returns only an error.
func Do(ctx context.Context, b Backoff, f RetryFunc) error {
	_, err := DoValue(ctx, b, func(ctx context.Context) (*struct{}, error) {
		return nil, f(ctx)
	})
	return err
}
