package timer

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/yola1107/kratos/v2/library/work/loop"
)

// Timer 定时器对象
type Timer struct {
	timerMap  sync.Map     // 存储任务ID对应的停止通道 [int64]chan struct{}
	idCounter atomic.Int64 // 原子递增的任务ID计数器
}

// New 创建新定时器实例
func New() *Timer {
	return &Timer{}
}

// Stop 停止指定ID的任务
func (t *Timer) Stop(timerID int64) {
	if stopFunc, ok := t.timerMap.LoadAndDelete(timerID); ok {
		if fn, ok := stopFunc.(func()); ok {
			fn()
		}
	}
}

// StopAll 停止所有定时任务
func (t *Timer) StopAll() {
	t.timerMap.Range(func(key, value interface{}) bool {
		if fn, ok := value.(func()); ok {
			fn()
		}
		return true
	})
	t.timerMap = sync.Map{}
	t.idCounter.Store(0)
}

// Once 执行一次定时任务
func (t *Timer) Once(loop loop.ILoop, duration time.Duration, f func()) int64 {
	return t.run(loop, duration, duration, false, f)
}

// Forever 固定间隔重复执行
func (t *Timer) Forever(loop loop.ILoop, interval time.Duration, f func()) int64 {
	return t.run(loop, interval, interval, true, f)
}

// ForeverNow 立即执行后按间隔重复
func (t *Timer) ForeverNow(loop loop.ILoop, interval time.Duration, f func()) int64 {
	if loop != nil {
		loop.Post(f)
	} else {
		f()
	}
	return t.Forever(loop, interval, f)
}

// ForeverTime 首次延迟与后续间隔不同的定时任务
func (t *Timer) ForeverTime(loop loop.ILoop, durFirst, durRepeat time.Duration, f func()) int64 {
	return t.run(loop, durFirst, durRepeat, true, f)
}

// 核心执行方法
func (t *Timer) run(loop loop.ILoop, durFirst, durRepeat time.Duration, repeated bool, f func()) int64 {
	timerID := t.idCounter.Add(1)
	stopCh := make(chan struct{})

	// 存储停止函数到全局map
	stopFunc := func() { close(stopCh) }
	t.timerMap.Store(timerID, stopFunc)

	// 启动定时任务协程
	go func() {
		defer t.timerMap.Delete(timerID)

		timer := time.NewTimer(durFirst)
		defer timer.Stop()

		for {
			select {
			case <-stopCh:
				return
			case <-timer.C:
				// 执行任务
				if loop != nil {
					loop.Post(f)
				} else {
					f()
				}
				if !repeated {
					return
				}
				timer.Reset(durRepeat)
			}
		}
	}()

	return timerID
}
