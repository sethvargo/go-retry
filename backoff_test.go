package retry

import (
	"context"
	"testing"
	"time"
)

func ExampleBackoffFunc() {
	// Example backoff middleware that adds the provided duration t to the result.
	withShift := func(t time.Duration, next Backoff) BackoffFunc {
		return func() (time.Duration, bool) {
			val, stop := next.Next()
			if stop {
				return 0, true
			}
			return val + t, false
		}
	}

	// Middlewrap wrap another backoff:
	b, err := NewFibonacci(1 * time.Second)
	if err != nil {
		// handle error
	}

	ctx := context.Background()
	if err := Do(ctx, withShift(5*time.Second, b), func(ctx context.Context) error {
		// Actual retry logic here
		return nil
	}); err != nil {
		// handle error
	}
}

func TestWithJitter(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100_000; i++ {
		b := WithJitter(250*time.Millisecond, BackoffFunc(func() (time.Duration, bool) {
			return 1 * time.Second, false
		}))
		val, stop := b.Next()
		if stop {
			t.Errorf("should not stop")
		}

		if min, max := 750*time.Millisecond, 1250*time.Millisecond; val < min || val > max {
			t.Errorf("expected %v to be between %v and %v", val, min, max)
		}
	}
}

func ExampleWithJitter() {
	ctx := context.Background()
	fib, err := NewFibonacci(1 * time.Second)
	if err != nil {
		// handle err
	}

	err = Do(ctx, WithJitter(1*time.Second, fib), func(_ context.Context) error {
		// TODO: logic here
		return nil
	})
	_ = err
}

func TestWithJitterPercent(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100_000; i++ {
		b := WithJitterPercent(5, BackoffFunc(func() (time.Duration, bool) {
			return 1 * time.Second, false
		}))
		val, stop := b.Next()
		if stop {
			t.Errorf("should not stop")
		}

		if min, max := 950*time.Millisecond, 1050*time.Millisecond; val < min || val > max {
			t.Errorf("expected %v to be between %v and %v", val, min, max)
		}
	}
}

func ExampleWithJitterPercent() {
	ctx := context.Background()
	fib, err := NewFibonacci(1 * time.Second)
	if err != nil {
		// handle err
	}

	err = Do(ctx, WithJitterPercent(5, fib), func(_ context.Context) error {
		// TODO: logic here
		return nil
	})
	_ = err
}

func TestWithMaxRetries(t *testing.T) {
	t.Parallel()

	b := WithMaxRetries(3, BackoffFunc(func() (time.Duration, bool) {
		return 1 * time.Second, false
	}))

	// First 3 attempts succeed
	for i := 0; i < 3; i++ {
		val, stop := b.Next()
		if stop {
			t.Errorf("should not stop")
		}
		if val != 1*time.Second {
			t.Errorf("expected %v to be %v", val, 1*time.Second)
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

func ExampleWithMaxRetries() {
	ctx := context.Background()
	fib, err := NewFibonacci(1 * time.Second)
	if err != nil {
		// handle err
	}

	err = Do(ctx, WithMaxRetries(3, fib), func(_ context.Context) error {
		// TODO: logic here
		return nil
	})
	_ = err
}

func TestWithCappedDuration(t *testing.T) {
	t.Parallel()

	b := WithCappedDuration(3*time.Second, BackoffFunc(func() (time.Duration, bool) {
		return 5 * time.Second, false
	}))

	val, stop := b.Next()
	if stop {
		t.Errorf("should not stop")
	}
	if val != 3*time.Second {
		t.Errorf("expected %v to be %v", val, 3*time.Second)
	}
}

func ExampleWithCappedDuration() {
	ctx := context.Background()
	fib, err := NewFibonacci(1 * time.Second)
	if err != nil {
		// handle err
	}

	err = Do(ctx, WithCappedDuration(3*time.Second, fib), func(_ context.Context) error {
		// TODO: logic here
		return nil
	})
	_ = err
}

func TestWithMaxDuration(t *testing.T) {
	t.Parallel()

	b := WithMaxDuration(250*time.Millisecond, BackoffFunc(func() (time.Duration, bool) {
		return 1 * time.Second, false
	}))

	// Take once, within timeout.
	val, stop := b.Next()
	if stop {
		t.Error("should not stop")
	}

	if val > 250*time.Millisecond {
		t.Errorf("expected %v to be less than %v", val, 250*time.Millisecond)
	}

	time.Sleep(200 * time.Millisecond)

	// Take again, remainder contines
	val, stop = b.Next()
	if stop {
		t.Error("should not stop")
	}

	if val > 50*time.Millisecond {
		t.Errorf("expected %v to be less than %v", val, 50*time.Millisecond)
	}

	time.Sleep(50 * time.Millisecond)

	// Now we stop
	val, stop = b.Next()
	if !stop {
		t.Errorf("should stop")
	}
	if val != 0 {
		t.Errorf("expected %v to be %v", val, 0)
	}
}

func ExampleWithMaxDuration() {
	ctx := context.Background()
	fib, err := NewFibonacci(1 * time.Second)
	if err != nil {
		// handle err
	}

	err = Do(ctx, WithMaxDuration(5*time.Second, fib), func(_ context.Context) error {
		// TODO: logic here
		return nil
	})
	_ = err
}
