// Package retry provides helpers for retrying.
//
// This package defines flexible interfaces for retrying Go functions that may
// be flaky or eventually consistent. It abstracts the "backoff" (how long to
// wait between tries) and "retry" (execute the function again) mechanisms for
// maximum flexibility. You can define your own backoffs and middleware.
//
// The package is modeled after Go's built-in HTTP package, making it easy to
// customize the built-in backoff with your own custom logic. Additionally,
// callers specify which errors are retryable by wrapping them. This is helpful
// with complex operations where only certain results should retry.
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

// RetryableError marks an error as retryable.
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
// the same context passed to f.
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

	for {
		// Return immediately if ctx is canceled
		if err := context.Cause(ctx); err != nil {
			return nilT, errors.Join(err, lastErr)
		}

		v, err := f(ctx)
		if err == nil {
			return v, nil
		}

		// Not retryable
		rerr, ok := errors.AsType[*retryableError](err)
		if !ok {
			return nilT, err
		}

		next, stop := b.Next()
		if stop {
			return nilT, rerr.Unwrap()
		}
		lastErr = rerr.Unwrap()

		// Wait until next attempt or until the context expires. Any error will
		// be caught at the top of the loop.
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

// Do wraps a function with a backoff to retry. The provided context is the same
// context passed to the [RetryFunc].
func Do(ctx context.Context, b Backoff, f RetryFunc) error {
	_, err := DoValue(ctx, b, func(ctx context.Context) (*struct{}, error) {
		return nil, f(ctx)
	})
	return err
}
