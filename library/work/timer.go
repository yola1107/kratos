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
	Len() int       // 注册在调度系统的任务数量
	Running() int32 // 当前活跃执行中的任务数
	Once(delay time.Duration, f func()) int64
	Forever(interval time.Duration, f func()) int64
	ForeverNow(interval time.Duration, f func()) int64
	Cancel(taskID int64)
	CancelAll()
	Shutdown()
}

type ITaskExecutor interface {
	Post(job func())
}

type preciseEvery struct {
	Interval time.Duration
	last     atomic.Value
}

func (p *preciseEvery) Next(t time.Time) time.Time {
	last, _ := p.last.Load().(time.Time)
	if last.IsZero() {
		last = t
	}
	next := last.Add(p.Interval)
	for !next.After(t) {
		next = next.Add(p.Interval)
	}
	p.last.Store(next)
	return next
}

type taskEntry struct {
	timer     *timingwheel.Timer
	cancelled atomic.Bool
	repeated  bool
}

type timingWheelScheduler struct {
	tw       *timingwheel.TimingWheel
	executor ITaskExecutor

	tasks     sync.Map // map[int64]*taskEntry
	nextID    atomic.Int64
	running   atomic.Int32 // 正在运行的活跃任务计数
	wg        sync.WaitGroup
	shutdown  atomic.Bool
	closeOnce sync.Once

	ctx    context.Context
	cancel context.CancelFunc
}

func NewTaskScheduler(exec ITaskExecutor, parentCtx context.Context) ITaskScheduler {
	// 创建时间轮 (1ms精度，512槽位)
	tw := timingwheel.NewTimingWheel(1*time.Millisecond, 512)
	ctx, cancel := context.WithCancel(parentCtx)

	s := &timingWheelScheduler{
		tw:       tw,
		executor: exec,
		ctx:      ctx,
		cancel:   cancel,
	}

	go func() {
		tw.Start()
		<-ctx.Done()
		tw.Stop()
	}()

	return s
}

func (s *timingWheelScheduler) Len() int {
	count := 0
	s.tasks.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

func (s *timingWheelScheduler) Running() int32 {
	return s.running.Load()
}

func (s *timingWheelScheduler) Once(delay time.Duration, f func()) int64 {
	return s.schedule(delay, false, f)
}

func (s *timingWheelScheduler) Forever(interval time.Duration, f func()) int64 {
	return s.schedule(interval, true, f)
}

func (s *timingWheelScheduler) ForeverNow(interval time.Duration, f func()) int64 {
	s.executeAsync(f)
	return s.schedule(interval, true, f)
}

func (s *timingWheelScheduler) Cancel(taskID int64) {
	s.removeTask(taskID)
}

func (s *timingWheelScheduler) CancelAll() {
	s.tasks.Range(func(key, _ any) bool {
		s.removeTask(key.(int64))
		return true
	})
}

func (s *timingWheelScheduler) Shutdown() {
	s.closeOnce.Do(func() {
		s.shutdown.Store(true)
		s.cancel() // 先停止时间轮，防止新任务触发
		s.CancelAll()

		// 等待所有任务完成
		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// 正常完成
		case <-time.After(100 * time.Millisecond):
			log.Warn("Scheduler shutdown timed out, some tasks may still be running")
		}
	})
}

func (s *timingWheelScheduler) schedule(delay time.Duration, repeated bool, f func()) int64 {
	if s.shutdown.Load() || s.ctx.Err() != nil {
		log.Warnf("Scheduler is shutdown, cannot schedule new task")
		return -1
	}

	// 生成唯一任务ID
	taskID := s.nextID.Add(1)
	entry := &taskEntry{repeated: repeated}
	s.tasks.Store(taskID, entry)

	wrapped := func() {
		// 检查任务是否已取消
		if entry.cancelled.Load() {
			return
		}

		// 在任务触发时增加计数
		s.running.Add(1)
		s.wg.Add(1)

		// 通过线程池执行器执行任务
		s.executeAsync(func() {
			// 确保计数减少
			defer s.running.Add(-1)
			defer s.wg.Done()

			// 再次检查取消状态
			if entry.cancelled.Load() {
				return
			}

			// 执行任务
			f()

			// 单次任务执行后立即移除
			if !repeated {
				s.removeTask(taskID)
			}
		})
	}

	if repeated {
		entry.timer = s.tw.ScheduleFunc(&preciseEvery{Interval: delay}, wrapped)
	} else {
		entry.timer = s.tw.AfterFunc(delay, wrapped)
	}

	return taskID
}

func (s *timingWheelScheduler) removeTask(taskID int64) {
	value, ok := s.tasks.Load(taskID)
	if !ok {
		return
	}
	entry := value.(*taskEntry)

	// 原子标记任务为已取消
	if !entry.cancelled.CompareAndSwap(false, true) {
		return
	}

	if entry.timer != nil {
		entry.timer.Stop()
	}
	s.tasks.Delete(taskID)
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
