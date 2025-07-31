package xgo

import (
	"runtime/debug"

	"github.com/yola1107/kratos/v2/log"
)

func RecoverFromError(cb func(e any)) {
	if e := recover(); e != nil {
		log.Errorf("Recover => %v\n%s\n", e, debug.Stack())
		if cb != nil {
			cb(e)
		}
	}
}
