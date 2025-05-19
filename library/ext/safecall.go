package ext

import (
	"github.com/yola1107/kratos/v2/log"
)

// SafeCall 安全执行回调
func SafeCall(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("handler panic: %v", r)
		}
	}()
	if fn != nil {
		fn()
	}
}
