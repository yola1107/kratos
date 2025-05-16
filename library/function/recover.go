package function

import (
	"runtime/debug"

	"github.com/yola1107/kratos/v2/log"
)

func RecoverFromError(cb func()) {
	if e := recover(); e != nil {
		log.Error("Recover => %s:%s\n", e, debug.Stack())
		if cb != nil {
			cb()
		}
	}
}
