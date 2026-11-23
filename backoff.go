package retry

import (
	"math"
	"math/rand/v2"
	"time"
)

// Backoff returns how long to wait before the next retry, or stop=true to stop
// retrying. [Do] passes the number of retries made so far, starting at 0. A
// Backoff that depends only on attempt holds no state, so one value can be
// reused across [Do] calls and shared between goroutines.
type Backoff func(attempt uint64) (next time.Duration, stop bool)

// WithJitter wraps a backoff function and adds the specified jitter. j can be
// interpreted as "+/- j". For example, if j were 5 seconds and the backoff
// returned 20s, the value could be between 15 and 25 seconds. The value can
// never be less than 0 or greater than the maximum time.Duration.
func WithJitter(j time.Duration, next Backoff) Backoff {
	return func(attempt uint64) (time.Duration, bool) {
		val, stop := next(attempt)
		if stop {
			return 0, true
		}

		if j <= 0 {
			return max(val, 0), false
		}

		// The unsigned bound fits even when twice j exceeds MaxInt64.
		diff := time.Duration(rand.Uint64N(uint64(j)*2) - uint64(j))
		return addClamp(val, diff), false
	}
}

// WithJitterPercent wraps a backoff function and adds the specified jitter
// percentage. j can be interpreted as "+/- j%". For example, if j were 5 and
// the backoff returned 20s, the value could be between 19 and 21 seconds. j is
// clamped to 100.
func WithJitterPercent(j uint64, next Backoff) Backoff {
	// Above 100 the random factor can go negative.
	j = min(j, 100)

	return func(attempt uint64) (time.Duration, bool) {
		val, stop := next(attempt)
		if stop {
			return 0, true
		}

		if j == 0 {
			return max(val, 0), false
		}

		pct := 1 + (rand.Float64()*2-1)*float64(j)/100
		return durationFromFloat(float64(val) * pct), false
	}
}

// WithFullJitter wraps a backoff function and returns a random value between
// zero (inclusive) and the wrapped backoff's returned value (exclusive). For
// example, if the backoff returned 20s, the value could be anywhere in [0, 20s).
//
// Unlike WithJitter, which applies "+/- j" around the value, full jitter
// spreads the wait across the entire range, which spreads out retrying clients
// more effectively. See
// https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/.
func WithFullJitter(next Backoff) Backoff {
	return func(attempt uint64) (time.Duration, bool) {
		val, stop := next(attempt)
		if stop {
			return 0, true
		}

		if val <= 0 {
			return 0, false
		}

		return time.Duration(rand.Int64N(int64(val))), false
	}
}

// WithMaxRetries executes the backoff function up until the maximum retries.
// Note this is retries, not attempts. Attempts is retries + 1.
func WithMaxRetries(maxRetries uint64, next Backoff) Backoff {
	return func(attempt uint64) (time.Duration, bool) {
		if attempt >= maxRetries {
			return 0, true
		}

		val, stop := next(attempt)
		if stop {
			return 0, true
		}

		return max(val, 0), false
	}
}

// WithCappedDuration sets a maximum on the duration returned from the next
// backoff. This is NOT a total backoff time, but rather a cap on the maximum
// value a backoff can return. Without another middleware, the backoff will
// continue infinitely. To limit the total time, use [context.WithTimeout] on
// the context passed to [Do].
func WithCappedDuration(limit time.Duration, next Backoff) Backoff {
	return func(attempt uint64) (time.Duration, bool) {
		val, stop := next(attempt)
		if stop {
			return 0, true
		}

		return max(min(val, limit), 0), false
	}
}

func addClamp(a, b time.Duration) time.Duration {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64
	}
	if b < 0 && a < math.MinInt64-b {
		return 0
	}
	return max(a+b, 0)
}

func durationFromFloat(f float64) time.Duration {
	if f >= math.MaxInt64 {
		return math.MaxInt64
	}
	if f <= 0 {
		return 0
	}
	return time.Duration(f)
}
