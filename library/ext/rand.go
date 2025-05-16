package ext

import (
	"math/rand"
	"time"

	"golang.org/x/exp/constraints"
)

var srand *rand.Rand

func init() {
	srand = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func GetRand() *rand.Rand {
	return srand
}

func IsHit(v int) bool {
	return srand.Intn(100) <= v
}

func IsHitFloat(v float64) bool {
	return RandFloat(0, 1.0) <= v
}

func RandFloat[T constraints.Float](min T, max T) T {
	if max <= min {
		return min
	}
	return T(srand.Float64())*(max-min) + min
}

func RandInt[T constraints.Integer](min T, max T) T {
	if max <= min {
		return min
	}
	return T(srand.Int63n(int64(max-min))) + min
}
