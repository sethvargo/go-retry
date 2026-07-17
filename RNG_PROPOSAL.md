# Proposal: replace the mutex-wrapped `math/rand` jitter source with lock-free `math/rand/v2`

## Summary

`WithJitter` / `WithJitterPercent` currently draw randomness through `rand.go`'s
`lockedSource` — a `sync.Mutex` wrapped around a `*math/rand.Rand`. That mutex
serializes every jitter draw across goroutines. Switching to the package-level
`math/rand/v2` functions (safe for concurrent use, no explicit lock) makes the
jitter path **~3× faster single-threaded and ~85× faster under concurrency**,
and lets the entire `rand.go` `lockedSource` machinery be deleted.

This is a standalone performance change, intentionally kept **out of the
Nix/static-analysis PR** so that one stays focused on the jitter panic fix and
`IsRetryable`.

## Motivation

`math/rand.Rand` is not safe for concurrent use, and `Backoff.Next()` is
documented as safe for concurrent use, so the package wraps the generator in a
mutex (`rand.go`):

```go
func (r *lockedSource) Int63n(n int64) int64 {
	if n <= 0 { panic("invalid argument to Int63n") }
	if n&(n-1) == 0 { return r.Int63() & (n - 1) } // r.Int63() locks
	max := int64((1 << 63) - 1 - (1<<63)%uint64(n))
	v := r.Int63()                                  // locks
	for v > max { v = r.Int63() }                   // locks per rejection
	return v % n
}
```

Every draw takes the lock (the rejection loop re-locks per sample), so
concurrent callers contend on a single mutex. `math/rand/v2` (GA since Go 1.22;
we already require Go 1.25) provides top-level `Int64N` that is safe for
concurrent use without an explicit lock and uses a faster generator (ChaCha8).

## Benchmark evidence

`AMD Ryzen Threadripper PRO 3945WX`, `-benchmem`, `-cpu=8`:

| Draw | 1 goroutine | 8 goroutines |
|---|---|---|
| `lockedSource.Int63n` (current) | 21.8 ns/op | **83.4 ns/op** (contends — gets *worse*) |
| `math/rand/v2.Int64N` (proposed) | 7.5 ns/op | **0.98 ns/op** (scales per-core) |

Both are 0 allocs/op. Public-path confirmation:

| Backoff | current 1g | current 8g |
|---|---|---|
| `WithJitter(...).Next()` | 19.3 ns/op | 84.3 ns/op |
| `WithJitterPercent(...).Next()` | 28.1 ns/op | — |

The parallel `WithJitter` number (84 ns) is dominated by the same mutex.

## Proposed change

### 1. Delete `rand.go`

The whole `lockedSource` type (`Int63`, `Seed`, `Uint64`, `Int63n`, the
`rand.Source64` assertion, `newLockedRandom`) becomes unused and is removed.

### 2. `backoff.go` — use the package-level v2 generator

```diff
 import (
+	"math/rand/v2"
 	"sync"
 	"time"
 )

 func WithJitter(j time.Duration, next Backoff) Backoff {
-	r := newLockedRandom(time.Now().UnixNano())
-
 	return BackoffFunc(func() (time.Duration, bool) {
 		val, stop := next.Next()
 		if stop {
 			return 0, true
 		}

 		// A jitter of zero (or negative) is a no-op. Guard it explicitly:
-		// Int63n panics when its argument is <= 0 ...
+		// Int64N panics when its argument is <= 0 ...
 		if j <= 0 {
 			return val, false
 		}

-		diff := time.Duration(r.Int63n(int64(j)*2) - int64(j))
+		diff := time.Duration(rand.Int64N(int64(j)*2) - int64(j))
 		val = val + diff
 		...
 	})
 }
```

…and the identical substitution in `WithJitterPercent`
(`r.Int63n(int64(j)*2)` → `rand.Int64N(int64(j)*2)`, drop the `newLockedRandom`
line). The zero-jitter guards added in the prior PR remain necessary — `Int64N`
still panics on `n <= 0`.

### 3. `README.md` Notes

> - Randomization uses `math/rand/v2` (auto-seeded) instead of `crypto/rand`.

## Compatibility & risk

- **No public API change.** `lockedSource` is unexported and used only by the
  two jitter wrappers; nothing consumes it as a `rand.Source`.
- **No behavioral regression.** The old code seeded each source with
  `time.Now().UnixNano()` (effectively random per process) and exposed no
  deterministic-seeding API. The v2 global generator is auto-seeded per process
  with equal-or-better randomness. Jitter remains statistically unchanged
  (uniform in `[-j, +j]`).
- **Concurrency.** `math/rand/v2`'s top-level `Int64N` is documented safe for
  concurrent use, preserving the `Backoff.Next()` concurrency contract that the
  mutex previously guaranteed.
- **gosec.** `math/rand/v2` is still flagged G404 (non-crypto). That was already
  a justified exclusion — jitter does not need cryptographic randomness — and is
  unchanged by this proposal.
- **Go version.** Requires Go ≥ 1.22 for `math/rand/v2`; go.mod is already 1.25.

## Optional follow-on (not required)

If deterministic jitter is ever wanted for tests, add an opt-in
`WithJitterSource(src *rand.Rand, ...)` variant rather than reintroducing a
mutex on the default path. Out of scope here.

## Suggested commit message

```
Use math/rand/v2 for jitter, drop mutex-wrapped rand source

WithJitter/WithJitterPercent drew randomness through a sync.Mutex-wrapped
math/rand.Rand (rand.go's lockedSource), serializing every draw across
goroutines. Switch to the package-level math/rand/v2 generator, which is
safe for concurrent use without an explicit lock.

~3x faster single-threaded and ~85x faster under 8-way concurrency for the
jitter path; deletes rand.go entirely. No public API or behavioral change
(seeding was already time-based and unexported).
```
