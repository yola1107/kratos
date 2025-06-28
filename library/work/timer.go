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

type preciseEvery struct {
	Interval time.Duration
	last     time.Time
}

func (p *preciseEvery) Next(t time.Time) time.Time {
	if p.last.IsZero() {
		p.last = t
	}
	p.last = p.last.Add(p.Interval)
	return p.last
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
	shutdown  atomic.Bool
	wg        sync.WaitGroup
	closeOnce sync.Once

	ctx    context.Context
	cancel context.CancelFunc
}

func NewTaskScheduler(exec ITaskExecutor, parentCtx context.Context) ITaskScheduler {
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
	s.tasks.Range(func(key, value any) bool {
		id := key.(int64)
		entry := value.(*taskEntry)
		entry.cancelled.Store(true)
		if entry.timer != nil {
			entry.timer.Stop()
		}
		s.tasks.Delete(id)
		return true
	})
}

func (s *timingWheelScheduler) Shutdown() {
	s.closeOnce.Do(func() {
		s.shutdown.Store(true)
		s.CancelAll()
		s.cancel()
		s.wg.Wait()
	})
}

func (s *timingWheelScheduler) schedule(delay time.Duration, repeated bool, f func()) int64 {
	if s.shutdown.Load() || s.ctx.Err() != nil {
		return -1
	}

	taskID := s.nextID.Add(1)
	entry := &taskEntry{repeated: repeated}

	var once sync.Once // 确保清理只执行一次

	wrapped := func() {
		if entry.cancelled.Load() {
			return
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()

			if entry.cancelled.Load() {
				return
			}

			s.executeAsync(f)

			if !repeated {
				once.Do(func() {
					s.removeTask(taskID)
				})
			}
		}()
	}

	if repeated {
		entry.timer = s.tw.ScheduleFunc(&preciseEvery{Interval: delay}, wrapped)
	} else {
		entry.timer = s.tw.AfterFunc(delay, wrapped)
	}

	s.tasks.Store(taskID, entry)
	return taskID
}

func (s *timingWheelScheduler) removeTask(taskID int64) {
	value, ok := s.tasks.Load(taskID)
	if !ok {
		return
	}
	entry := value.(*taskEntry)

	// 使用 atomic.CompareAndSwapBool 防止重复取消
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
