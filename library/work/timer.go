package work

import (
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/yola1107/kratos/v2/log"
)

// Scheduler 任务调度器接口
type Scheduler interface {
	Len() int                                          // 当前注册任务数量
	Running() int32                                    // 当前正在执行的任务数量
	Monitor() Monitor                                  // 获取任务池状态信息
	Once(delay time.Duration, f func()) int64          // 注册一次性任务
	Forever(interval time.Duration, f func()) int64    // 注册周期任务
	ForeverNow(interval time.Duration, f func()) int64 // 注册周期任务并立即执行一次
	Cancel(taskID int64)                               // 取消指定任务
	CancelAll()                                        // 取消所有任务
	Stop()                                             // 停止调度器
}

// IExecutor 可选的自定义执行器接口（如线程池）
type IExecutor interface {
	Post(job func())
}

// Monitor 任务池状态信息
type Monitor struct {
	Capacity int   // 容量（堆调度器使用，时间轮调度器为0）
	Len      int   // 当前注册任务数量
	Running  int32 // 当前执行中的任务数量
}

const (
	// maxIntervalJumps 防止周期任务因为滞后而无限补跑
	maxIntervalJumps = 10000
)

// RecoverFromError 任务执行错误恢复
func RecoverFromError(cb func(e any)) {
	if e := recover(); e != nil {
		log.Errorf("Recover => %v\n%s\n", e, debug.Stack())
		if cb != nil {
			cb(e)
		}
	}
}

// ExecuteAsync 通用异步任务执行函数
func ExecuteAsync(executor IExecutor, f func()) {
	run := func() {
		defer RecoverFromError(nil)
		f()
	}
	if executor != nil {
		executor.Post(run)
	} else {
		go run()
	}
}

// --- 通用基础结构 ---

// baseScheduler 提供通用调度器功能的基础实现
type baseScheduler struct {
	executor IExecutor
	running  atomic.Int32
}

func (s *baseScheduler) executeAsync(f func()) {
	ExecuteAsync(s.executor, f)
}

func (s *baseScheduler) incrementRunning() {
	s.running.Add(1)
}

func (s *baseScheduler) decrementRunning() {
	s.running.Add(-1)
}

func (s *baseScheduler) getRunning() int32 {
	return s.running.Load()
}
