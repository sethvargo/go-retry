package retry

import (
	"math"
	"math/rand/v2"
	"time"
)

// Backoff returns the wait before the next retry, or stop=true to end
// retrying. [Do] passes the number of retries made so far, starting at 0. The
// built-in backoffs and middleware depend only on attempt, so one value can be
// reused across [Do] calls and shared between goroutines.
type Backoff func(attempt uint64) (next time.Duration, stop bool)

// WithJitter adds a random amount in [-j, j) to each wait from next. For
// example, with j = 5s and a 20s wait, the result is between 15s and 25s. The
// result is never below 0 or above math.MaxInt64.
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

// WithJitterPercent multiplies each wait from next by a random factor in
// [1-j/100, 1+j/100). For example, with j = 5 and a 20s wait, the result is
// between 19s and 21s. A j above 100 counts as 100.
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

// WithFullJitter replaces each wait from next with a random value in [0, wait).
// For example, a 20s wait becomes anything from 0 up to just under 20s. Retries
// from many clients end up further apart than with [WithJitter]. See
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

// WithCappedDuration limits each wait from next to at most limit. It returns
// stop=true only when next does. To limit the total time spent retrying, use
// [context.WithTimeout] on the context passed to [Do].
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
