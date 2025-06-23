package ext

import (
	crand "crypto/rand"
	"encoding/binary"
	"math"
	"math/rand"
	"sync"
	"time"

	"golang.org/x/exp/constraints"
)

/*

	线程安全的真实随机
*/

var randPool = sync.Pool{
	New: func() any {
		return rand.New(rand.NewSource(betterSeed()))
	},
}

func betterSeed() int64 {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		return time.Now().UnixNano()
	}
	return int64(binary.LittleEndian.Uint64(b[:])) ^ time.Now().UnixNano()
}

func getRand() *rand.Rand {
	return randPool.Get().(*rand.Rand)
}

func putRand(r *rand.Rand) {
	randPool.Put(r)
}

func IsHit(v int) bool {
	if v <= 0 {
		return false
	}
	if v >= 100 {
		return true
	}
	r := getRand()
	defer putRand(r)
	return r.Intn(100) < v
}

func IsHitFloat(v float64) bool {
	if v <= 0 {
		return false
	}
	if v >= 1 {
		return true
	}
	r := getRand()
	defer putRand(r)
	return r.Float64() < v
}

func RandFloat(min, max float64) float64 {
	if max <= min {
		return min
	}
	r := getRand()
	defer putRand(r)
	return r.Float64()*(max-min) + min
}

func RandInt[T constraints.Integer](min, max T) T {
	if max <= min {
		return min
	}
	diff := uint64(max - min)
	if diff > math.MaxInt64 {
		return min
	}
	r := getRand()
	defer putRand(r)
	return min + T(r.Int63n(int64(diff)))
}

func RandIntInclusive[T constraints.Integer](min, max T) T {
	if max < min {
		return min
	}
	diff := uint64(max - min + 1)
	if diff > math.MaxInt64 {
		return min
	}
	r := getRand()
	defer putRand(r)
	return min + T(r.Int63n(int64(diff)))
}
