package retry

import (
	"math/rand"
	"sync"
	"time"
)

var r = &lockedSource{src: rand.New(rand.NewSource(time.Now().UnixNano()))}

type lockedSource struct {
	lk  sync.Mutex
	src *rand.Rand
}

var _ rand.Source = (*lockedSource)(nil)

// Int63 mimics math/rand.(*Rand).Int63 with mutex locked.
func (r *lockedSource) Int63() int64 {
	r.lk.Lock()
	n := r.src.Int63()
	r.lk.Unlock()
	return n
}

// Seed mimics math/rand.(*Rand).Seed with mutex locked.
func (r *lockedSource) Seed(seed int64) {
	r.lk.Lock()
	r.src.Seed(seed)
	r.lk.Unlock()
}

// Int63n mimics math/rand.(*Rand).Int63n with mutex locked.
func (r *lockedSource) Int63n(n int64) int64 {
	if n <= 0 {
		panic("invalid argument to Int63n")
	}
	if n&(n-1) == 0 { // n is power of two, can mask
		return r.Int63() & (n - 1)
	}
	max := int64((1 << 63) - 1 - (1<<63)%uint64(n))
	v := r.Int63()
	for v > max {
		v = r.Int63()
	}
	return v % n
}
