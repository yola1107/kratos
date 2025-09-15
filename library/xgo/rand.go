package xgo

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

// IsHit 判断概率为 v% 的事件是否命中 (v 范围 [0,100])
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

// IsHitFloat 判断概率为 v 的事件是否命中 (v 范围 [0,1])
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

// RandFloat 生成 [min, max) 范围的随机浮点数
func RandFloat(min, max float64) float64 {
	if max <= min {
		return min
	}
	r := getRand()
	defer putRand(r)
	return r.Float64()*(max-min) + min
}

// RandInt 生成 [min, max) 范围的随机整数
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

// RandIntInclusive 生成 [min, max] 范围的随机整数
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
