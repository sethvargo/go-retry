package retry_test

// Benchmarks for the backoff hot paths, focused on the jitter draw whose cost
// is dominated by the RNG. Run:
//
//	go test -run '^$' -bench 'WithJitter' -benchmem -cpu=1,8
//
// The Parallel variants expose the mutex contention in the current
// lockedSource-based implementation; a lock-free math/rand/v2 draw removes it.

import (
	"testing"
	"time"

	retry "github.com/sethvargo/go-retry"
)

// benchSink prevents the compiler from optimizing away the Next() calls.
var benchSink time.Duration

func BenchmarkWithJitter(b *testing.B) {
	bo := retry.WithJitter(100*time.Millisecond, retry.NewConstant(1*time.Second))
	b.ReportAllocs()
	b.ResetTimer()
	var d time.Duration
	for i := 0; i < b.N; i++ {
		d, _ = bo.Next()
	}
	benchSink = d
}

func BenchmarkWithJitterParallel(b *testing.B) {
	bo := retry.WithJitter(100*time.Millisecond, retry.NewConstant(1*time.Second))
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var d time.Duration
		for pb.Next() {
			d, _ = bo.Next()
		}
		benchSink = d
	})
}

func BenchmarkWithJitterPercent(b *testing.B) {
	bo := retry.WithJitterPercent(5, retry.NewConstant(1*time.Second))
	b.ReportAllocs()
	b.ResetTimer()
	var d time.Duration
	for i := 0; i < b.N; i++ {
		d, _ = bo.Next()
	}
	benchSink = d
}

func BenchmarkWithJitterPercentParallel(b *testing.B) {
	bo := retry.WithJitterPercent(5, retry.NewConstant(1*time.Second))
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var d time.Duration
		for pb.Next() {
			d, _ = bo.Next()
		}
		benchSink = d
	})
}
