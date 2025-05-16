package function

import (
	"fmt"
	"log"
	"strconv"
)

// ToString .
func ToString(v any) string {
	return fmt.Sprintf("%v", v)
}

func IntToStr(src int) string {
	return strconv.Itoa(src)
}

func Int32ToStr(src int32) string {
	return strconv.Itoa(int(src))
}

func Int64ToStr(src int64) string {
	return strconv.FormatInt(src, 10)
}

func Float64ToStr(f float64, prec ...int) string {
	p := 2
	if len(prec) > 0 {
		p = prec[0]
	}
	return strconv.FormatFloat(f, 'f', p, 64)
}

func StrToInt(src string) int {
	if src == "" {
		return 0
	}
	dst, err := strconv.Atoi(src)
	if err != nil {
		log.Printf("str to int error(%v).", err)
		return 0
	}
	return dst
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

func StrToFloat64(src string) float64 {
	if src == "" {
		return 0
	}
	if v, err := strconv.ParseFloat(src, 64); err == nil {
		return v
	}
	return 0
}
