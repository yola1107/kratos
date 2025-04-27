package gtimer

import (
	"time"

	"github.com/yola1107/kratos/v2/library/task"
)

/*
	全局定时器
*/

func Once(loop *task.Loop, duration time.Duration, f func()) {
	run(loop, duration, duration, false, f)
}

func Forever(loop *task.Loop, duration time.Duration, f func()) {
	run(loop, duration, duration, true, f)
}

func ForeverNow(loop *task.Loop, duration time.Duration, f func()) {
	if loop != nil {
		loop.Post(f)
	} else {
		f()
	}
	Forever(loop, duration, f)
}

func ForeverTime(loop *task.Loop, durFirst, durRepeat time.Duration, f func()) {
	run(loop, durFirst, durRepeat, true, f)
}

func run(loop *task.Loop, durFirst, durRepeat time.Duration, repeated bool, f func()) {
	go func() {
		timer := time.NewTimer(durFirst)
		for {
			select {
			case <-timer.C:
				if loop != nil {
					loop.Post(f)
				} else {
					f()
				}
				if repeated {
					timer.Reset(durRepeat)
				} else {
					return
				}
			}
		}
	}()
}
