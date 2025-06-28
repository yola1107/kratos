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
	last     atomic.Value // 使用atomic.Value确保线程安全
}

func (p *preciseEvery) Next(t time.Time) time.Time {
	last, _ := p.last.Load().(time.Time)
	if last.IsZero() {
		last = t
	}
	next := last.Add(p.Interval)

	// 减少因时间漂移导致的任务堆积
	// 确保返回的时间在未来
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
		s.removeTask(id)
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
			log.Warnf("Shutdown timed out waiting for tasks")
		}
	})
}

func (s *timingWheelScheduler) schedule(delay time.Duration, repeated bool, f func()) int64 {
	if s.shutdown.Load() || s.ctx.Err() != nil {
		log.Warnf("Scheduler is shutdown, cannot schedule new task")
		return -1
	}

	taskID := s.nextID.Add(1)
	entry := &taskEntry{repeated: repeated}
	s.tasks.Store(taskID, entry) // 先存储任务再启动计时器

	var once sync.Once // 确保清理只执行一次

	wrapped := func() {
		// 在任务触发时立即检查取消状态
		if entry.cancelled.Load() {
			return
		}

		s.wg.Add(1) // 在任务触发时增加计数
		s.executeAsync(func() {
			defer s.wg.Done() // 确保计数减少

			// 执行前再次检查取消状态
			if entry.cancelled.Load() {
				return
			}

			// 放入到线程池安全执行
			s.executeAsync(f)

			// 一次性任务执行后自动清理
			if !repeated {
				once.Do(func() {
					s.removeTask(taskID)
				})
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

	// 使用原子操作确保只取消一次
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
