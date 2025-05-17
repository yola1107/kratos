package timer

import (
	"context"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/log"
)

/*
	定时器任务
*/

// ITaskScheduler 核心调度接口
type ITaskScheduler interface {
	Once(duration time.Duration, f func()) int64
	Forever(interval time.Duration, f func()) int64
	ForeverNow(interval time.Duration, f func()) int64
	ForeverTime(durFirst, durRepeat time.Duration, f func()) int64
	Stop(taskID int64)
	StopAll()
}

// TaskScheduler  定时器任务
type taskScheduler struct {
	seq   atomic.Int64    // 原子递增的任务ID计数器
	tasks sync.Map        // 存储任务ID对应的停止通道 [int64]context.CancelFunc
	loop  ILoop           // 任务池执行器
	ctx   context.Context // 根上下文
}

// NewTaskScheduler 创建新定时器实例
func NewTaskScheduler(loop ILoop) ITaskScheduler {
	return &taskScheduler{
		loop: loop,
		ctx:  context.Background(), // 可传入外部Context
	}
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

// Stop 停止指定ID的任务
func (t *taskScheduler) Stop(taskID int64) {
	if cancel, ok := t.tasks.LoadAndDelete(taskID); ok {
		if cancelFn, ok := cancel.(context.CancelFunc); ok {
			cancelFn() // 取消特定任务
		}
	}
}

// StopAll 停止所有定时任务
func (t *taskScheduler) StopAll() {
	t.tasks.Range(func(key, value any) bool {
		if cancelFn, ok := value.(context.CancelFunc); ok {
			cancelFn()
		}
		t.tasks.Delete(key)
		return true
	})
}

// 核心执行方法
func (t *taskScheduler) run(durFirst, durRepeat time.Duration, repeated bool, f func()) int64 {
	taskID := t.seq.Add(1)
	ctx, cancel := context.WithCancel(t.ctx) // 派生Context
	t.tasks.Store(taskID, cancel)

	// 启动定时任务协程
	go func() {
		defer t.tasks.Delete(taskID)
		defer cancel()

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
				timer.Reset(durRepeat)
			}
		}
	}()

	return taskID
}

func safeCall(loop ILoop, f func()) {
	if loop != nil {
		loop.Post(func() {
			defer recoverFromError()
			f()
		})
	} else {
		go func() {
			defer recoverFromError()
			f()
		}()
	}
}

func recoverFromError() {
	if e := recover(); e != nil {
		log.Error("Recover => %s:%s\n", e, debug.Stack())
	}
}

/*
	任务池 job pool
*/

type ILoop interface {
	Start()
	Stop()
	Jobs() int
	Post(job func())
	PostAndWait(job func() any) any
}

type taskBuffer struct {
	jobs    chan func()
	toggle  chan byte
	once    sync.Once
	stopped atomic.Bool
}

// NewTaskBuffer 创建一个Loop队列，max为队列最大任务数量长度
func NewTaskBuffer(jobsCnt int) ILoop {
	return &taskBuffer{
		jobs:   make(chan func(), jobsCnt),
		toggle: make(chan byte),
	}
}

func (lp *taskBuffer) Start() {
	log.Infof("loop start ..")
	go func() {
		defer ext.RecoverFromError(func() {
			lp.Start()
		})
		for {
			select {
			case <-lp.toggle:
				lp.stopped.Store(true)
				log.Info("loop routine stop. Remaining(%d)", lp.Jobs())
				return
			case job := <-lp.jobs:
				job()
			}
		}
	}()
}

func (lp *taskBuffer) Stop() {
	lp.once.Do(
		func() {
			go func() {
				close(lp.toggle)
			}()
		})
}

func (lp *taskBuffer) Jobs() int {
	return len(lp.jobs)
}

func (lp *taskBuffer) Post(job func()) {
	if lp.stopped.Load() {
		return
	}
	go func() {
		lp.jobs <- job
	}()
}

func (lp *taskBuffer) PostAndWait(job func() any) any {
	if lp.stopped.Load() {
		return nil
	}
	ch := make(chan any)
	go func() {
		lp.jobs <- func() {
			ch <- job()
		}
	}()
	return <-ch
}
