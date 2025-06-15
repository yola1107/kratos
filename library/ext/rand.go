package ext

import (
	"math/rand"
	"time"
)

var (
	secureRand = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// IsHit 判断概率为 v% 的事件是否命中 (v 范围 [0,100])
func IsHit(v int) bool {
	if v <= 0 {
		return false
	}
	if v >= 100 {
		return true
	}
	return secureRand.Intn(100) < v
}

// IsHitFloat 判断概率为 v 的事件是否命中 (v 范围 [0,1])
func IsHitFloat(v float64) bool {
	if v <= 0 {
		return false
	}
	if v >= 1 {
		return true
	}
	return secureRand.Float64() < v
}

// RandFloat 生成 [min, max) 范围的随机浮点数
func RandFloat(min, max float64) float64 {
	if max <= min {
		return min
	}
	return secureRand.Float64()*(max-min) + min
}

// RandInt 生成 [min, max] 范围的随机整数
func RandInt(min, max int) int {
	if max <= min {
		return min
	}
	return min + secureRand.Intn(max-min+1)
}
