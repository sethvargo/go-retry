package retry

import (
	"errors"
	"math/rand"
	"sync"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Backoff is an interface that backs off.
type Backoff interface {
	// Next takes the error and returns the time duration to wait and the
	// processed error. A duration less than zero signals the backoff to stop
	// and to not retry again.
	Next(err error) (time.Duration, error)
}

// BackoffFunc is a backoff expressed as a function.
type BackoffFunc func(err error) (time.Duration, error)

// Next implements Backoff.
func (b BackoffFunc) Next(err error) (time.Duration, error) {
	return b(err)
}

// Stop value signals the backoff to stop retrying.
const Stop = time.Duration(-1)

// IsStopped reports whether the backoff shall stop.
func IsStopped(delay time.Duration) bool {
	return delay < 0
}

// WithJitter wraps a backoff function and adds the specified jitter.
// If addOnly is specified, then a jitter up to +j will be added on top of the
// backoff; otherwise a jitter up to ±j will be applied. For example, if j is
// 5s, addOnly is false and the backoff returned is 20s, then the resulting
// value could be between 15 and 25 seconds. Panics if j is less than 0.
func WithJitter(j time.Duration, addOnly bool, next Backoff) Backoff {
	if j < 0 {
		panic("jitter must be >= 0")
	}
	return BackoffFunc(func(err error) (time.Duration, error) {
		delay, err := next.Next(err)
		if IsStopped(delay) {
			return Stop, err
		}

		if addOnly {
			delay += time.Duration(rand.Int63n(int64(j)))
		} else {
			diff := time.Duration(rand.Int63n(int64(j)*2) - int64(j))
			delay = delay + diff
			if delay < 0 {
				delay = 0
			}
		}
		return delay, err
	})
}

// WithJitterPercent wraps a backoff function and adds the specified jitter
// percentage.
// If addOnly is specified, then a jitter up to +j% will be added on top of the
// backoff; otherwise a jitter up to ±j% will be applied. For example, if j is
// 5, addOnly is false and the backoff returned is 20s, then the resulting
// value could be between 19 and 21 seconds. Panics if j is less than 0 or greater than 100.
func WithJitterPercent(j uint64, addOnly bool, next Backoff) Backoff {
	if j < 0 && j > 100 {
		panic("jitter must be between 0 and 100")
	}
	return BackoffFunc(func(err error) (time.Duration, error) {
		delay, err := next.Next(err)
		if IsStopped(delay) {
			return Stop, err
		}

		var top int64
		if addOnly {
			top = rand.Int63n(int64(j))
		} else {
			// get random value between -j and +j
			top = rand.Int63n(int64(j)*2) - int64(j)
		}
		pct := 1 + float64(top)/100.0

		delay = time.Duration(float64(delay) * pct)
		if delay < 0 {
			delay = 0
		}
		return delay, err
	})
}

// WithMaxRetries executes the backoff function up until the maximum attempts.
func WithMaxRetries(max uint64, next Backoff) Backoff {
	var l sync.Mutex
	var attempt uint64

	return BackoffFunc(func(err error) (time.Duration, error) {
		l.Lock()
		defer l.Unlock()

		if attempt >= max {
			return Stop, err
		}
		attempt++

		return next.Next(err)
	})
}

// WithCappedDuration sets a maximum on the duration returned from the next
// backoff. This is NOT a total backoff time, but rather a cap on the maximum
// value a backoff can return. Without another middleware, the backoff will
// continue infinitely.
func WithCappedDuration(cap time.Duration, next Backoff) Backoff {
	return BackoffFunc(func(err error) (time.Duration, error) {
		delay, err := next.Next(err)
		if IsStopped(delay) {
			return Stop, err
		}

		if delay <= 0 || delay > cap {
			delay = cap
		}
		return delay, err
	})
}

// WithMaxDuration sets a maximum on the total amount of time a backoff should
// execute. It's best-effort, and should not be used to guarantee an exact
// amount of time.
func WithMaxDuration(timeout time.Duration, next Backoff) Backoff {
	start := time.Now()

	return BackoffFunc(func(err error) (time.Duration, error) {
		diff := timeout - time.Since(start)
		if diff <= 0 {
			return Stop, err
		}

		delay, err := next.Next(err)
		if IsStopped(delay) {
			return Stop, err
		}

		if delay <= 0 || delay > diff {
			delay = diff
		}
		return delay, err
	})
}

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

// WithRetryable wraps a backoff function and adds a check for a RetryableError.
// When a non RetryableError then no more retry is performed.
func WithRetryable(next Backoff) Backoff {
	return BackoffFunc(func(err error) (time.Duration, error) {
		var rerr *retryableError
		if !errors.As(err, &rerr) {
			return Stop, err
		}
		return next.Next(rerr.Unwrap())
	})
}
