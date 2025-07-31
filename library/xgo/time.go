package xgo

import (
	"time"
)

// GetTick 毫秒级时间戳
func GetTick() int64 {
	return time.Now().UnixNano() / 1e6
}

// GetCurSec 获取当前时间 s
func GetCurSec() int64 {
	return time.Now().Unix()
}

// GetCurNanoSec 获取当前纳秒
func GetCurNanoSec() int64 {
	return time.Now().UnixNano()
}

// GetCurDate 返回当前的日期（零时零分）
func GetCurDate() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

func FormatDateTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func FormatTime(t time.Time, layout string) string {
	return t.Format(layout)
}
