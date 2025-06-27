package work

import (
	"context"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RussellLuo/timingwheel"
	"github.com/yola1107/kratos/v2/log"
)

type ITaskScheduler interface {
	Len() int
	Once(duration time.Duration, f func()) int64
	Forever(interval time.Duration, f func()) int64
	ForeverNow(interval time.Duration, f func()) int64
	Cancel(taskID int64)
	CancelAll()
	Shutdown()
}

type ITaskExecutor interface {
	Post(job func())
}

type Every struct{ Interval time.Duration }

func (e *Every) Next(t time.Time) time.Time {
	if e.Interval <= 0 {
		return time.Time{}
	}
	return t.Add(e.Interval)
}

type timingWheelScheduler struct {
	tw       *timingwheel.TimingWheel
	executor ITaskExecutor

	mu        sync.Mutex
	tasks     map[int64]*taskEntry
	nextID    int64
	shutdown  bool
	wg        sync.WaitGroup
	closeOnce sync.Once

	ctx    context.Context
	cancel context.CancelFunc
}

type taskEntry struct {
	timer     *timingwheel.Timer
	cancelled atomic.Bool // 原子操作确保并发安全
	repeated  bool
}

func NewTaskScheduler(exec ITaskExecutor, parentCtx context.Context) ITaskScheduler {
	tw := timingwheel.NewTimingWheel(1*time.Millisecond, 512)
	ctx, cancel := context.WithCancel(parentCtx)

	s := &timingWheelScheduler{
		tw:       tw,
		executor: exec,
		ctx:      ctx,
		cancel:   cancel,
		tasks:    make(map[int64]*taskEntry),
	}

	go func() {
		tw.Start()
		<-ctx.Done()
		tw.Stop()
	}()

	return s
}

// Len 返回当前活跃任务数量
func (s *timingWheelScheduler) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.tasks)
}

// Once 执行一次定时任务
func (s *timingWheelScheduler) Once(delay time.Duration, f func()) int64 {
	return s.schedule(delay, false, f)
}

// Forever 固定间隔重复执行
func (s *timingWheelScheduler) Forever(interval time.Duration, f func()) int64 {
	return s.schedule(interval, true, f)
}

// ForeverNow 立即执行后按间隔重复
func (s *timingWheelScheduler) ForeverNow(interval time.Duration, f func()) int64 {
	s.executeAsync(f)
	return s.schedule(interval, true, f)
}

// Cancel 停止指定ID的定时任务
func (s *timingWheelScheduler) Cancel(taskID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelTaskLocked(taskID)
}

// CancelAll 停止所有定时任务
func (s *timingWheelScheduler) CancelAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.tasks {
		s.cancelTaskLocked(id)
	}
}

func (s *timingWheelScheduler) Shutdown() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.shutdown = true

		// 取消所有任务
		for id := range s.tasks {
			s.cancelTaskLocked(id)
		}
		s.mu.Unlock()

		// 取消上下文，停止时间轮
		s.cancel()

		// 等待所有执行中的任务完成
		s.wg.Wait()
	})
}

// 核心执行方法
func (s *timingWheelScheduler) schedule(delay time.Duration, repeated bool, f func()) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查调度器状态
	if s.shutdown || s.ctx.Err() != nil {
		return -1
	}

	s.nextID++
	taskID := s.nextID

	entry := &taskEntry{repeated: repeated}
	s.tasks[taskID] = entry

	// 创建任务包装函数（在时间轮的goroutine中执行）
	wrapped := func() {
		// 在任务触发时检查取消状态
		if entry.cancelled.Load() {
			return
		}

		// 关键：在创建 goroutine 之前增加计数
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()

			// 在实际执行前再次检查取消状态
			if entry.cancelled.Load() {
				return
			}

			s.executeAsync(f)

			// 一次性任务执行后自动清理
			if !repeated {
				s.mu.Lock()
				s.cancelTaskLocked(taskID)
				s.mu.Unlock()
			}
		}()
	}

	if repeated {
		// 周期性任务
		entry.timer = s.tw.ScheduleFunc(&Every{Interval: delay}, wrapped)
	} else {
		// 一次性任务
		entry.timer = s.tw.AfterFunc(delay, wrapped)
	}

	return taskID
}

func (s *timingWheelScheduler) cancelTaskLocked(taskID int64) {
	entry, ok := s.tasks[taskID]
	if !ok {
		return
	}

	if entry.timer != nil {
		entry.timer.Stop()
	}
	entry.cancelled.Store(true)
	delete(s.tasks, taskID)
}

// executeAsync 把 f 投递到任务池或直接 goroutine，并带 recover
func (s *timingWheelScheduler) executeAsync(f func()) {
	if s.executor != nil {
		s.executor.Post(func() {
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

func RecoverFromError(cb func(e any)) {
	if e := recover(); e != nil {
		log.Errorf("Recover => %v\n%s\n", e, debug.Stack())
		if cb != nil {
			cb(e)
		}
	}
}
