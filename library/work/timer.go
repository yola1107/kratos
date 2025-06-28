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

const (
	// 创建时间轮 (默认100ms精度，512槽位, 跨度0.1*512=51.2s)
	defaultTickPrecision = 100 * time.Millisecond // 时间轮精度
	defaultWheelSize     = int64(512)             // 时间轮槽位
)

type ITaskScheduler interface {
	Len() int       // 注册在调度系统的任务数量
	Running() int32 // 当前活跃执行中的任务数
	Once(delay time.Duration, f func()) int64
	Forever(interval time.Duration, f func()) int64
	ForeverNow(interval time.Duration, f func()) int64
	Cancel(taskID int64)
	CancelAll()
	Stop()
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

type SchedulerOption func(*timingWheelScheduler)

// WithTick 设置时间轮 tick 精度 tick最小1ms 精度越小越消耗cpu
func WithTick(tick time.Duration) SchedulerOption {
	return func(cfg *timingWheelScheduler) {
		cfg.tick = tick
	}
}

// WithWheelSize 设置时间轮槽数量
func WithWheelSize(size int64) SchedulerOption {
	return func(cfg *timingWheelScheduler) {
		cfg.wheelSize = size
	}
}

// WithContext 设置上下文
func WithContext(ctx context.Context) SchedulerOption {
	return func(cfg *timingWheelScheduler) {
		cfg.ctx = ctx
	}
}

// WithExecutor 设置任务执行器
func WithExecutor(exec ITaskExecutor) SchedulerOption {
	return func(cfg *timingWheelScheduler) {
		cfg.executor = exec
	}
}

type timingWheelScheduler struct {
	tick      time.Duration
	wheelSize int64
	executor  ITaskExecutor
	tw        *timingwheel.TimingWheel

	tasks     sync.Map // map[int64]*taskEntry
	nextID    atomic.Int64
	running   atomic.Int32 // 正在运行的活跃任务计数
	wg        sync.WaitGroup
	shutdown  atomic.Bool
	closeOnce sync.Once

	ctx    context.Context
	cancel context.CancelFunc
}

func NewTaskScheduler(opts ...SchedulerOption) ITaskScheduler {
	s := &timingWheelScheduler{
		tick:      defaultTickPrecision,
		wheelSize: defaultWheelSize,
		ctx:       context.Background(),
		executor:  nil,
	}

	for _, opt := range opts {
		opt(s)
	}

	// 初始化定时轮
	s.tw = timingwheel.NewTimingWheel(s.tick, s.wheelSize)

	// 创建带取消的上下文
	ctx, cancel := context.WithCancel(s.ctx)
	s.ctx = ctx
	s.cancel = cancel

	// 启动时间轮，并监听上下文退出
	go func() {
		s.tw.Start()
		<-ctx.Done()
		s.tw.Stop()
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

func (s *timingWheelScheduler) removeTask(taskID int64) {
	value, ok := s.tasks.Load(taskID)
	if !ok {
		return
	}
	entry := value.(*taskEntry)
	if !entry.cancelled.CompareAndSwap(false, true) {
		return
	}
	if entry.timer != nil {
		entry.timer.Stop()
	}
	s.tasks.Delete(taskID)
}

func (s *timingWheelScheduler) Stop() {
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
		case <-time.After(100 * time.Millisecond):
			log.Warn("Scheduler shutdown timed out, some tasks may still be running")
		}
	})
}

// schedule 定时任务核心方法
func (s *timingWheelScheduler) schedule(delay time.Duration, repeated bool, f func()) int64 {
	if s.shutdown.Load() || s.ctx.Err() != nil {
		log.Warnf("Scheduler is shutdown, cannot schedule new task")
		return -1
	}

	taskID := s.nextID.Add(1)
	entry := &taskEntry{repeated: repeated}

	wrapped := func() {
		if entry.cancelled.Load() {
			return
		}
		s.running.Add(1)
		s.wg.Add(1)

		s.executeAsync(func() {
			defer s.running.Add(-1)
			defer s.wg.Done()
			if entry.cancelled.Load() {
				return
			}
			f()
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

	// Move this after `entry` is fully initialized
	s.tasks.Store(taskID, entry)
	return taskID
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
