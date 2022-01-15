package benchmark

import (
	"context"
	"math"
	"testing"
	"time"

	cenkalti "github.com/cenkalti/backoff"
	lestrrat "github.com/lestrrat-go/backoff"
	sethvargo "github.com/sethvargo/go-retry"
)

func Benchmark(b *testing.B) {
	b.Run("cenkalti", func(b *testing.B) {
		backoff := cenkalti.NewExponentialBackOff()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			backoff.NextBackOff()
		}
	})

	b.Run("lestrrat", func(b *testing.B) {
		policy := lestrrat.NewExponential(
			lestrrat.WithFactor(0),
			lestrrat.WithInterval(0),
			lestrrat.WithJitterFactor(0),
			lestrrat.WithMaxRetries(math.MaxInt64),
		)
		backoff, cancel := policy.Start(context.Background())
		defer cancel()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			select {
			case <-backoff.Done():
				b.Fatalf("ended early")
			case <-backoff.Next():
			}
		}
	})

	b.Run("sethvargo", func(b *testing.B) {
		backoff := sethvargo.NewExponential(1 * time.Second)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			backoff.Next()
		}
	})
}
