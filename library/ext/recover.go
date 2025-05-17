package ext

import (
	"runtime/debug"

	"github.com/yola1107/kratos/v2/log"
)

func RecoverFromError(cb func()) {
	if e := recover(); e != nil {
		log.Errorf("Recover => %v:%s\n", e, debug.Stack())
		if cb != nil {
			cb()
		}
	}
}
