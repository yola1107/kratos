package base

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strconv"
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

func RandFloat(min float64, max float64) float64 {
	return srand.Float64()*(max-min) + min
}

func RandInt[T constraints.Integer](min T, max T) T {
	if max-min <= 0 {
		return min
	}
	num := srand.Int63n(int64(max - min))

	return min + T(num)
}

func RandWeighted[T constraints.Integer](weighted []T) int {
	total := T(0)
	ats := []T{}
	for _, v := range weighted {
		total += v
		ats = append(ats, total)
	}
	rnd := RandInt(0, total)
	for i, v := range ats {
		if rnd < v {
			return i
		}
	}
	return 0
}

//ToString .
func ToString(v any) string {
	return fmt.Sprintf("%v", v)
}

//ToJSON json string
func ToJSON(v any) string {
	j, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(j)
}

func StrToInt(src string) int {
	dst, err := strconv.Atoi(src)
	if err != nil {
		log.Printf("str to int error(%v).", err)
		return 0
	}
	return dst
}

func StrToInt64(src string) int64 {
	if src == "" {
		return 0
	}
	dst, err := strconv.ParseInt(src, 10, 64)
	if err != nil {
		log.Printf("str to int64 error(%v).", err)
		return 0
	}
	return dst
}

func IntToStr(src int) string {
	return strconv.Itoa(src)
}

func Int64ToStr(src int64) string {
	return strconv.FormatInt(src, 10)
}

func StrToInt32(src string) int32 {
	if src == "" {
		return 0
	}
	dst, err := strconv.Atoi(src)
	if err != nil {
		log.Printf("str to int32 error(%v).", err)
		return 0
	}
	return int32(dst)
}

func Int32ToStr(src int32) string {
	return strconv.Itoa(int(src))
}

// 工具函数：格式化float64为字符串，保留两位小数
func Float64ToStr(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}

func StrToFloat64(s string) float64 {
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	return 0
}
