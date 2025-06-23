package ext

import (
	"math/rand"
	"sync"
	"time"

	"golang.org/x/exp/constraints"
)

var (
	randPool = sync.Pool{
		New: func() any {
			// 用纳秒时间戳 + goroutine id/hash作为种子更均匀
			seed := time.Now().UnixNano() ^ int64(rand.Intn(1<<20))
			return rand.New(rand.NewSource(seed))
		},
	}
)

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
func RandInt[T constraints.Integer](min T, max T) T {
	if max <= min {
		return min
	}
	r := getRand()
	defer putRand(r)
	num := r.Int63n(int64(max - min))
	return min + T(num)
}

// RandIntInclusive 生成 [min, max] 范围的随机整数
func RandIntInclusive[T constraints.Integer](min T, max T) T {
	if max <= min {
		return min
	}
	r := getRand()
	defer putRand(r)
	num := r.Int63n(int64(max - min + 1))
	return min + T(num)
}
