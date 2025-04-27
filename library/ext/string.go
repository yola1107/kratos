package ext

import (
	"fmt"
	"strconv"

	"golang.org/x/exp/constraints"

	"github.com/yola1107/kratos/v2/log"
)

// ToString .
func ToString(v any) string {
	return fmt.Sprintf("%v", v)
}

func IntToStr(n int) string {
	return strconv.FormatInt(int64(n), 10)
}

func Int32ToStr(n int32) string {
	return strconv.FormatInt(int64(n), 10)
}

func Int64ToStr(n int64) string {
	return strconv.FormatInt(int64(n), 10)
}

func Float64ToStr(f float64, prec ...int) string {
	p := 2
	if len(prec) > 0 {
		p = prec[0]
	}
	return strconv.FormatFloat(f, 'f', p, 64)
}

func StrToInt(s string) int {
	return strToInt[int](s)
}

func StrToInt32(s string) int32 {
	return strToInt[int32](s)
}

func StrToInt64(s string) int64 {
	return strToInt[int64](s)
}

func StrToFloat64(s string) float64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Warnf("str to float64 error(%v).", err)
	}
	return v
}

func strToInt[T constraints.Integer](s string) T {
	if len(s) == 0 {
		return 0
	}
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		log.Warnf("str to int error(%v).", err)
		return 0
	}
	return T(val)
}
