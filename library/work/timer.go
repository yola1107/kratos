package work

import (
	"context"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yola1107/kratos/v2/log"
)

/*
	定时器任务
*/

const (
	defaultTickPrecision = 100 * time.Millisecond
	defaultWheelSize     = int64(512)
)

// ITaskScheduler 核心调度接口
type ITaskScheduler interface {
	Len() int // 当前活跃定时任务数量
	Running() int32
	Once(duration time.Duration, f func()) int64
	Forever(interval time.Duration, f func()) int64
	ForeverNow(interval time.Duration, f func()) int64
	ForeverTime(durFirst, durRepeat time.Duration, f func()) int64
	Cancel(taskID int64)
	CancelAll()
	Stop()
}

// ITaskExecutor 任务执行器接口
type ITaskExecutor interface {
	Post(job func())
}

type SchedulerOption func(*taskScheduler)

func WithContext(ctx context.Context) SchedulerOption {
	return func(cfg *taskScheduler) {
		cfg.ctx = ctx
	}
}

func WithExecutor(exec ITaskExecutor) SchedulerOption {
	return func(cfg *taskScheduler) {
		cfg.executor = exec
	}
}

// taskScheduler  定时器任务
type taskScheduler struct {
	count    atomic.Int64    // 当前活跃任务数量
	seq      atomic.Int64    // 原子递增的任务ID计数器
	tasks    sync.Map        // 存储任务ID对应的停止通道 [int64]context.CancelFunc
	executor ITaskExecutor   // 任务池执行器
	ctx      context.Context // 根上下文
	cancel   context.CancelFunc
}

// NewTaskScheduler 创建新定时器实例
func NewTaskScheduler(opts ...SchedulerOption) ITaskScheduler {
	s := &taskScheduler{
		executor: nil,
		ctx:      context.Background(),
	}
	for _, opt := range opts {
		opt(s)
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.ctx = ctx
	s.cancel = cancel
	return s
}

// Len 返回当前活跃任务数量
func (t *taskScheduler) Len() int {
	return int(t.count.Load())
}

func (t *taskScheduler) Running() int32 {
	// 返回当前正在执行的任务数量
	// 注意：这不是活跃任务总数，而是当前正在执行的任务数
	return 0 // 示例值，实际实现需要跟踪
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
	safeCall(t.executor, f)
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

func (t *taskScheduler) Stop() {
	t.CancelAll()
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
				safeCall(t.executor, f)
				if !repeated {
					return
				}
				timer.Reset(durRepeat)
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

// RecoverFromError 恢复panic
func RecoverFromError(cb func(e any)) {
	if e := recover(); e != nil {
		log.Errorf("Recover => %v\n%s\n", e, debug.Stack())
		if cb != nil {
			cb(e)
		}
	}
}
