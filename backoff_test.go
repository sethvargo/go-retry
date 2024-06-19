package retry

import (
	"testing"
	"time"
)

func TestWithJitter(t *testing.T) {
	t.Parallel()

	baseDuration := 1 * time.Second
	backoffJitter := 250 * time.Millisecond

	sawJitter := false
	for i := 0; i < 100_000; i++ {
		b := WithJitter(backoffJitter, BackoffFunc(func() (time.Duration, bool) {
			return baseDuration, false
		}))
		val, stop := b.Next()
		if stop {
			t.Errorf("should not stop")
		}

		if val != baseDuration {
			sawJitter = true
		}

		if min, max := baseDuration-backoffJitter, baseDuration+backoffJitter; val < min || val > max {
			t.Errorf("expected %v to be between %v and %v", val, min, max)
		}
	}

	if !sawJitter {
		t.Fatal("expected to see jitter, all values were the same")
	}
}

func TestWithJitterPercent(t *testing.T) {
	t.Parallel()

	baseDuration := 1 * time.Second
	jitterPercent := uint64(5)
	minBackoff := time.Duration(100-jitterPercent) * baseDuration / 100
	maxBackoff := time.Duration(100+jitterPercent) * baseDuration / 100

	sawJitter := false
	for i := 0; i < 100_000; i++ {
		b := WithJitterPercent(jitterPercent, BackoffFunc(func() (time.Duration, bool) {
			return baseDuration, false
		}))
		val, stop := b.Next()
		if stop {
			t.Errorf("should not stop")
		}

		if val != baseDuration {
			sawJitter = true
		}

		if val < minBackoff || val > maxBackoff {
			t.Errorf("expected %v to be between %v and %v", val, minBackoff, maxBackoff)
		}
	}

	if !sawJitter {
		t.Fatal("expected to see jitter, all values were the same")
	}
}

func TestWithMaxRetries(t *testing.T) {
	t.Parallel()

	baseDuration := 1 * time.Second
	maxRetries := uint64(3)
	b := WithMaxRetries(maxRetries, BackoffFunc(func() (time.Duration, bool) {
		return baseDuration, false
	}))

	// First 3 attempts succeed
	for i := uint64(0); i < maxRetries; i++ {
		val, stop := b.Next()
		if stop {
			t.Errorf("should not stop")
		}
		if val != baseDuration {
			t.Errorf("expected %v to be %v", val, baseDuration)
		}
	}

	// Now we stop
	val, stop := b.Next()
	if !stop {
		t.Errorf("should stop")
	}
	if val != 0 {
		t.Errorf("expected %v to be %v", val, 0)
	}
}

func TestWithCappedDuration(t *testing.T) {
	t.Parallel()

	baseDuration := 5 * time.Second
	cappedDuration := 3 * time.Second
	b := WithCappedDuration(cappedDuration, BackoffFunc(func() (time.Duration, bool) {
		return baseDuration, false
	}))

	val, stop := b.Next()
	if stop {
		t.Errorf("should not stop")
	}
	if val != cappedDuration {
		t.Errorf("expected %v to be %v", val, cappedDuration)
	}
}

func TestWithMaxDuration(t *testing.T) {
	t.Parallel()

	baseDuration := 1 * time.Second
	maxDuration := 250 * time.Millisecond
	b := WithMaxDuration(maxDuration, BackoffFunc(func() (time.Duration, bool) {
		return baseDuration, false
	}))

	validateMaxDuration(t, b, maxDuration)
}

func TestResettableBackoff(t *testing.T) {
	var attempt uint64
	b := WithReset(func() {
		attempt = 0
	}, BackoffFunc(func() (time.Duration, bool) {
		attempt++
		return time.Duration(attempt) * time.Second, false
	}))

	// Call Next a few times
	for i := 0; i < 3; i++ {
		val, stop := b.Next()
		if stop {
			t.Fatal("should not stop")
		}
		if val != time.Duration(i+1)*time.Second {
			t.Errorf("expected %v to be %v", val, time.Duration(i+1)*time.Second)
		}
	}

	// Reset the backoff
	b.Reset()

	// Call Next again and verify that the state has been reset
	val, stop := b.Next()
	if stop {
		t.Fatal("should not stop after reset")
	}
	if val != time.Second {
		t.Errorf("expected %v to be %v", val, time.Second)
	}
}

func TestResettableBackoff_WithJitter(t *testing.T) {
	t.Parallel()

	baseDuration := 1 * time.Second
	jitterDuration := 1 * time.Second
	b := WithJitter(jitterDuration, BackoffFunc(func() (time.Duration, bool) {
		return baseDuration, false
	}))

	// reset it and verify that we are still within the jitter range
	b.reset()

	sawJitter := false
	for i := 0; i < 100_000; i++ {
		val, stop := b.Next()
		if stop {
			t.Errorf("should not stop")
		}

		if val != 1*time.Second {
			sawJitter = true
		}
		if min, max := baseDuration-jitterDuration, baseDuration+jitterDuration; val < min || val > max {
			t.Errorf("expected %v to be between %v and %v", val, min, max)
		}
	}

	if !sawJitter {
		t.Fatal("expected to see jitter, all values were the same")
	}
}

func TestResettableBackoff_WithJitterPercent(t *testing.T) {
	t.Parallel()

	baseDuration := 1 * time.Second
	jitterPercent := uint64(5)
	minBackoff := time.Duration(100-jitterPercent) * baseDuration / 100
	maxBackoff := time.Duration(100+jitterPercent) * baseDuration / 100
	b := WithJitterPercent(jitterPercent, BackoffFunc(func() (time.Duration, bool) {
		return baseDuration, false
	}))

	// reset it and verify that we are still within the jitter range
	b.reset()

	sawJitter := false
	for i := 0; i < 100_000; i++ {
		val, stop := b.Next()
		if stop {
			t.Errorf("should not stop")
		}

		if val != 1*time.Second {
			sawJitter = true
		}
		if val < minBackoff || val > maxBackoff {
			t.Errorf("expected %v to be between %v and %v", val, minBackoff, maxBackoff)
		}
	}

	if !sawJitter {
		t.Fatal("expected to see jitter, all values were the same")
	}
}

func TestResettableBackoff_WithMaxRetries(t *testing.T) {
	t.Parallel()

	baseDuration := 1 * time.Second
	maxRetries := uint64(3)
	b := WithMaxRetries(maxRetries, BackoffFunc(func() (time.Duration, bool) {
		return baseDuration, false
	}))

	// First 3 attempts succeed
	for i := uint64(0); i < maxRetries; i++ {
		val, stop := b.Next()
		if stop {
			t.Errorf("should not stop")
		}
		if val != baseDuration {
			t.Errorf("expected %v to be %v", val, baseDuration)
		}
	}

	b.reset()

	// reset - should get 3 more succeessful attempts
	for i := uint64(0); i < maxRetries; i++ {
		val, stop := b.Next()
		if stop {
			t.Errorf("should not stop after reset")
		}
		if val != baseDuration {
			t.Errorf("expected %v to be %v", val, baseDuration)
		}
	}

	// Now we stop
	val, stop := b.Next()
	if !stop {
		t.Errorf("should stop")
	}
	if val != 0 {
		t.Errorf("expected %v to be %v", val, 0)
	}
}

func TestResettableBackoff_WithCappedDuration(t *testing.T) {
	t.Parallel()

	baseDuration := 5 * time.Second
	cappedDuration := 3 * time.Second
	b := WithCappedDuration(cappedDuration, BackoffFunc(func() (time.Duration, bool) {
		return baseDuration, false
	}))

	val, stop := b.Next()
	if stop {
		t.Errorf("should not stop")
	}
	if val != cappedDuration {
		t.Errorf("expected %v to be %v", val, cappedDuration)
	}

	// verify that we still have cappedDuration after a reset
	b.reset()

	val, stop = b.Next()
	if stop {
		t.Errorf("should not stop")
	}
	if val != cappedDuration {
		t.Errorf("expected %v to be %v", val, cappedDuration)
	}
}

func validateMaxDuration(t *testing.T, b *ResettableBackoff, maxDuration time.Duration) {
	t.Helper()

	// Take once, within timeout.
	val, stop := b.Next()
	if stop {
		t.Error("should not stop")
	}

	if val > maxDuration {
		t.Errorf("expected %v to be less than %v", val, maxDuration)
	}

	// sleep for 80% of max duration
	longSleep80 := time.Duration(8) * maxDuration / 10
	time.Sleep(longSleep80)

	// Take again, remainder contines
	val, stop = b.Next()
	if stop {
		t.Error("should not stop")
	}

	// val should be <= 20% of max duration since we slept for 80% of it above
	shortSleep20 := time.Duration(2) * maxDuration / 10
	if val > shortSleep20 {
		t.Errorf("expected %v to be less than %v", val, shortSleep20)
	}

	time.Sleep(shortSleep20)

	// Now we stop
	val, stop = b.Next()
	if !stop {
		t.Errorf("should stop")
	}
	if val != 0 {
		t.Errorf("expected %v to be %v", val, 0)
	}
}

func TestResettableBackoff_WithMaxDuration(t *testing.T) {
	t.Parallel()

	baseDuration := 1 * time.Second
	maxDuration := 250 * time.Millisecond
	b := WithMaxDuration(maxDuration, BackoffFunc(func() (time.Duration, bool) {
		return baseDuration, false
	}))

	validateMaxDuration(t, b, maxDuration)

	// a reset should clear it, and we do the process again
	b.reset()

	validateMaxDuration(t, b, maxDuration)
}
