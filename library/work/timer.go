package work

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

/*
	定时器任务
*/

// ITaskScheduler 核心调度接口
type ITaskScheduler interface {
	Len() int // 当前活跃定时任务数量
	Once(duration time.Duration, f func()) int64
	Forever(interval time.Duration, f func()) int64
	ForeverNow(interval time.Duration, f func()) int64
	ForeverTime(durFirst, durRepeat time.Duration, f func()) int64
	Cancel(taskID int64)
	CancelAll()
}

type ITaskExecutor interface {
	Post(job func())
}

// taskScheduler  定时器任务
type taskScheduler struct {
	count atomic.Int64    // 当前活跃任务数量
	seq   atomic.Int64    // 原子递增的任务ID计数器
	tasks sync.Map        // 存储任务ID对应的停止通道 [int64]context.CancelFunc
	loop  ITaskExecutor   // 任务池执行器
	ctx   context.Context // 根上下文
}

// NewTaskScheduler 创建新定时器实例
func NewTaskScheduler(loop ITaskExecutor, ctx context.Context) ITaskScheduler {
	return &taskScheduler{
		loop: loop,
		ctx:  ctx,
	}
}

// Len 返回当前活跃任务数量
func (t *taskScheduler) Len() int {
	return int(t.count.Load())
}

// Once 执行一次定时任务
func (t *taskScheduler) Once(duration time.Duration, f func()) int64 {
	return t.run(duration, 0, false, f)
}

// Forever 固定间隔重复执行
func (t *taskScheduler) Forever(interval time.Duration, f func()) int64 {
	return t.run(interval, interval, true, f)
}

// ForeverNow 立即执行后按间隔重复
func (t *taskScheduler) ForeverNow(interval time.Duration, f func()) int64 {
	safeCall(t.loop, f)
	return t.Forever(interval, f)
}

// ForeverTime 首次延迟与后续间隔不同的定时任务
func (t *taskScheduler) ForeverTime(durFirst, durRepeat time.Duration, f func()) int64 {
	return t.run(durFirst, durRepeat, true, f)
}

// Cancel 停止指定ID的定时任务
func (t *taskScheduler) Cancel(taskID int64) {
	if cancel, ok := t.tasks.LoadAndDelete(taskID); ok {
		if cancelFn, ok := cancel.(context.CancelFunc); ok {
			cancelFn() // 取消特定任务
		}
	}
}

// CancelAll 停止所有定时任务
func (t *taskScheduler) CancelAll() {
	t.tasks.Range(func(_, value any) bool {
		if cancelFn, ok := value.(context.CancelFunc); ok {
			cancelFn()
		}
		return true
	})
}

// 核心执行方法
func (t *taskScheduler) run(durFirst, durRepeat time.Duration, repeated bool, f func()) int64 {
	taskID := t.seq.Add(1)
	ctx, cancel := context.WithCancel(t.ctx) // 派生Context
	t.tasks.Store(taskID, cancel)
	t.count.Add(1) // 统计 +1

	// 启动定时任务协程
	go func() {
		defer func() {
			t.tasks.Delete(taskID)
			cancel()
			t.count.Add(-1) // 统计 -1
		}()

		timer := time.NewTimer(durFirst)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done(): // 监听取消
				return
			case <-timer.C:
				safeCall(t.loop, f)
				if !repeated {
					return
				}
				safeReset(timer, durRepeat)
			}
		}
	}()

	return taskID
}

// safeCall 把 f 投递到任务池或直接 goroutine，并带 recover
func safeCall(loop ITaskExecutor, f func()) {
	if loop != nil {
		loop.Post(func() {
			defer RecoverFromError(nil)
			f()
		})
	} else {
		go func() {
			defer RecoverFromError(nil)
			f()
		}()
	}
}

// safeReset 安全地 reset timer，避免潜在 panic
func safeReset(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}
