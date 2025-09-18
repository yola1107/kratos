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
	defaultTickPrecision = 500 * time.Millisecond // 默认调度循环精度
	defaultWheelSize     = 128                    // 默认时间轮槽位
	maxIntervalJumps     = 10000
)

// ITaskScheduler 任务调度器接口
type ITaskScheduler interface {
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

// ITaskExecutor 可选的自定义执行器接口（如线程池）
type ITaskExecutor interface {
	Post(job func())
}

// Monitor 任务池状态信息
type Monitor struct {
	Len     int   // 当前注册任务数量
	Running int32 // 当前执行中的任务数量
}

// preciseEvery 实现精准的周期性定时器，防止时间漂移
type preciseEvery struct {
	Interval time.Duration
	last     atomic.Value // time.Time
}

func (p *preciseEvery) Next(t time.Time) time.Time {
	last, _ := p.last.Load().(time.Time)
	if last.IsZero() {
		last = t
	}
	steps := 0
	next := last.Add(p.Interval)
	for !next.After(t) {
		next = next.Add(p.Interval)
		if steps++; steps > maxIntervalJumps {
			log.Warnf("[preciseEvery] skipped too many steps: %d", steps)
			break
		}
	}
	p.last.Store(next)
	return next
}

// SchedulerOption 调度器选项
type SchedulerOption func(*Scheduler)

func WithTick(d time.Duration) SchedulerOption {
	return func(s *Scheduler) { s.tick = d }
}

func WithWheelSize(size int64) SchedulerOption {
	return func(s *Scheduler) { s.wheelSize = size }
}

func WithContext(ctx context.Context) SchedulerOption {
	return func(s *Scheduler) { s.ctx = ctx }
}

func WithExecutor(exec ITaskExecutor) SchedulerOption {
	return func(s *Scheduler) { s.executor = exec }
}

// Scheduler 定时任务调度器，基于时间轮实现
type Scheduler struct {
	tick      time.Duration            // 精度
	wheelSize int64                    // 槽位
	executor  ITaskExecutor            // 执行器 (如协程池)
	tw        *timingwheel.TimingWheel // 时间轮

	tasks    sync.Map     // map[int64]*taskEntry
	nextID   atomic.Int64 // 任务ID递增
	running  atomic.Int32 // 当前执行中任务数
	shutdown atomic.Bool  // 是否关闭
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	once     sync.Once
}

type taskEntry struct {
	timer     *timingwheel.Timer
	cancelled atomic.Bool
	repeated  bool
	executing atomic.Bool
	task      func()
}

// NewTaskScheduler 创建调度器实例
func NewTaskScheduler(opts ...SchedulerOption) ITaskScheduler {
	s := &Scheduler{
		tick:      defaultTickPrecision,
		wheelSize: defaultWheelSize,
		ctx:       context.Background(),
	}
	for _, opt := range opts {
		opt(s)
	}

	s.ctx, s.cancel = context.WithCancel(s.ctx)
	s.tw = timingwheel.NewTimingWheel(s.tick, s.wheelSize)
	go func() {
		s.tw.Start()
		<-s.ctx.Done()
		s.tw.Stop()
	}()
	return s
}

func (s *Scheduler) Len() int {
	count := 0
	s.tasks.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

func (s *Scheduler) Running() int32 {
	return s.running.Load()
}

func (s *Scheduler) Monitor() Monitor {
	return Monitor{
		Len:     s.Len(),
		Running: s.Running(),
	}
}

// Once 注册一次性任务
func (s *Scheduler) Once(delay time.Duration, f func()) int64 {
	return s.schedule(delay, false, f)
}

// Forever 注册周期任务
func (s *Scheduler) Forever(interval time.Duration, f func()) int64 {
	return s.schedule(interval, true, f)
}

// ForeverNow 注册周期任务并立即执行一次
func (s *Scheduler) ForeverNow(interval time.Duration, f func()) int64 {
	s.executeAsync(f)
	return s.schedule(interval, true, f)
}

// Cancel 取消指定任务
func (s *Scheduler) Cancel(taskID int64) {
	s.removeTask(taskID)
}

// CancelAll 取消所有任务
func (s *Scheduler) CancelAll() {
	s.tasks.Range(func(key, _ any) bool {
		s.removeTask(key.(int64))
		return true
	})
}

func (s *Scheduler) removeTask(taskID int64) {
	val, ok := s.tasks.Load(taskID)
	if !ok {
		return
	}
	entry := val.(*taskEntry)
	if entry.cancelled.CompareAndSwap(false, true) {
		if entry.timer != nil {
			entry.timer.Stop()
		}
		s.tasks.Delete(taskID)
	}
}

// Stop 停止调度器，等待正在执行任务完成
func (s *Scheduler) Stop() {
	s.once.Do(func() {
		s.shutdown.Store(true)
		s.cancel()
		s.CancelAll()
		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			log.Warn("scheduler shutdown timed out, some tasks may still be running")
		}
	})
}

// schedule 注册任务
func (s *Scheduler) schedule(delay time.Duration, repeated bool, f func()) int64 {
	if s.shutdown.Load() || s.ctx.Err() != nil {
		log.Warn("scheduler is shut down; task rejected")
		return -1
	}

	taskID := s.nextID.Add(1)
	entry := &taskEntry{repeated: repeated, task: f}
	startAt := time.Now()

	wrapped := func() {
		wrappedAt := time.Now()
		if entry.cancelled.Load() || !entry.executing.CompareAndSwap(false, true) {
			return
		}
		s.running.Add(1)
		s.wg.Add(1)

		s.executeAsync(func() {
			execAt := time.Now()
			defer func() {
				RecoverFromError(nil)
				s.wg.Done()
				s.running.Add(-1)
				entry.executing.Store(false)
				if !repeated {
					s.removeTask(taskID)
					s.lazy(taskID, delay, startAt, execAt, wrappedAt)
				}
			}()

			if entry.cancelled.Load() {
				return
			}
			f()
		})
	}

	if repeated {
		entry.timer = s.tw.ScheduleFunc(&preciseEvery{Interval: delay}, wrapped)
	} else {
		entry.timer = s.tw.AfterFunc(delay, wrapped)
	}

	s.tasks.Store(taskID, entry)
	return taskID
}

func (s *Scheduler) executeAsync(f func()) {
	wrapped := func() {
		select {
		case <-s.ctx.Done():
			return
		default:
			defer RecoverFromError(nil)
			f()
		}
	}
	if s.executor != nil {
		s.executor.Post(wrapped)
	} else {
		go wrapped()
	}
}

// log debug
func (s *Scheduler) lazy(taskID int64, delay time.Duration, startAt, execAt, wrappedAt time.Time) {
	now := time.Now()
	lazy := now.Sub(startAt)
	latency := lazy - delay

	if latency >= s.tick {
		exec, wrapped := now.Sub(execAt), now.Sub(wrappedAt)
		log.Errorf("[scheduler] taskID=%d delay=%v precision=%v lazy=%v latency=%v exec=%+v wrap=%+v",
			taskID, delay, s.tick, lazy, latency, exec, wrapped-exec,
		)
	}
}

// RecoverFromError 任务执行错误恢复
func RecoverFromError(cb func(e any)) {
	if e := recover(); e != nil {
		log.Errorf("Recover => %v\n%s\n", e, debug.Stack())
		if cb != nil {
			cb(e)
		}
	}
}
