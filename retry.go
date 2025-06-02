// Package retry provides helpers for retrying.
//
// This package defines flexible interfaces for retrying Go functions that may
// be flakey or eventually consistent. It abstracts the "backoff" (how long to
// wait between tries) and "retry" (execute the function again) mechanisms for
// maximum flexibility. Furthermore, everything is an interface, so you can
// define your own implementations.
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

// RetryFuncValue is a function passed to [Do] which returns a value.
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
	if e.err == nil {
		return "retryable: <nil>"
	}
	return "retryable: " + e.err.Error()
}

type retryCountKey struct{}

// GetRetryCount returns the number of retries so far. The first attempt will return 0.
func GetRetryCount(ctx context.Context) uint64 {
	retries, ok := ctx.Value(retryCountKey{}).(uint64)
	if !ok {
		retries = 0
	}
	return retries
}

func setRetryCount(ctx context.Context, retries uint64) context.Context {
	return context.WithValue(ctx, retryCountKey{}, retries)
}

func DoValue[T any](ctx context.Context, b Backoff, f RetryFuncValue[T]) (T, error) {
	var nilT T

	var retries uint64
	for {
		// Return immediately if ctx is canceled
		select {
		case <-ctx.Done():
			return nilT, ctx.Err()
		default:
		}

		// Ensure new context will go out of scope so it can get GC'd
		ctxWithRetryCount := setRetryCount(ctx, retries)
		v, err := f(ctxWithRetryCount)
		if err == nil {
			return v, nil
		}

		// Not retryable
		var rerr *retryableError
		if !errors.As(err, &rerr) {
			return nilT, err
		}

		next, stop := b.Next()
		if stop {
			return nilT, rerr.Unwrap()
		}

		// ctx.Done() has priority, so we test it alone first
		select {
		case <-ctx.Done():
			return nilT, ctx.Err()
		default:
		}

		t := time.NewTimer(next)
		select {
		case <-ctx.Done():
			t.Stop()
			return nilT, ctx.Err()
		case <-t.C:
			retries++
			continue
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
