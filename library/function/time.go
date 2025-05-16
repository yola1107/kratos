package function

import (
	"time"
)

// 获取当前秒
func GetCurSec() int64 {
	return time.Now().Unix()
}

// 获取当前纳秒
func GetCurNanoSec() int64 {
	return time.Now().UnixNano()
}

// 获取当前毫秒
func GetCurTicks() int64 {
	return time.Now().UnixNano() / 1e6
}

// 获取时间日期
func GetDate() time.Time {
	then := time.Date(
		2017, 06, 21, 20, 34, 58, 0, time.UTC)

	return then
}
