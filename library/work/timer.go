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

// IExecutor 任务执行器接口（如协程池）
type IExecutor interface {
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
type SchedulerOption func(*scheduler)

func WithTick(d time.Duration) SchedulerOption {
	return func(s *scheduler) {
		if d > 0 {
			s.tick = d
		} else {
			log.Warnf("Invalid tick %v, using default %v", d, defaultTickPrecision)
		}
	}
}

func WithWheelSize(size int64) SchedulerOption {
	return func(s *scheduler) {
		if size > 0 {
			s.wheelSize = size
		} else {
			log.Warnf("Invalid wheelSize %d, using default %d", size, defaultWheelSize)
		}
	}
}

func WithContext(ctx context.Context) SchedulerOption {
	return func(s *scheduler) { s.ctx = ctx }
}

func WithExecutor(exec IExecutor) SchedulerOption {
	return func(s *scheduler) { s.executor = exec }
}

func WithStopTimeout(timeout time.Duration) SchedulerOption {
	return func(s *scheduler) {
		if timeout > 0 {
			s.stopTimeout = timeout
		}
	}
}

// scheduler 定时任务调度器，基于时间轮实现
type scheduler struct {
	tick        time.Duration            // 精度
	wheelSize   int64                    // 槽位
	executor    IExecutor                // 执行器 (如协程池)
	tw          *timingwheel.TimingWheel // 时间轮
	stopTimeout time.Duration            // Stop 超时时间
	tasks       sync.Map                 // map[int64]*taskEntry
	nextID      atomic.Int64             // 任务ID递增
	running     atomic.Int32             // 当前执行中任务数
	shutdown    atomic.Bool              // 是否关闭
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	once        sync.Once
}

type taskEntry struct {
	timer     *timingwheel.Timer
	cancelled atomic.Bool
	repeated  bool
	executing atomic.Bool
	task      func()
}

// NewScheduler 创建调度器实例
func NewScheduler(opts ...SchedulerOption) Scheduler {
	s := &scheduler{
		tick:        defaultTickPrecision,
		wheelSize:   defaultWheelSize,
		ctx:         context.Background(),
		stopTimeout: 3 * time.Second, // 默认超时 3 秒
	}
	for _, opt := range opts {
		opt(s)
	}

	if s.executor == nil {
		log.Warn("[scheduler] No executor provided, tasks will run in unlimited goroutines")
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

func (s *scheduler) Len() int {
	count := 0
	s.tasks.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

func (s *scheduler) Running() int32 {
	return s.running.Load()
}

func (s *scheduler) Monitor() Monitor {
	return Monitor{
		Len:     s.Len(),
		Running: s.Running(),
	}
}

// Once 注册一次性任务
func (s *scheduler) Once(delay time.Duration, f func()) int64 {
	return s.schedule(delay, false, f)
}

// Forever 注册周期任务
func (s *scheduler) Forever(interval time.Duration, f func()) int64 {
	return s.schedule(interval, true, f)
}

// ForeverNow 注册周期任务并立即执行一次
func (s *scheduler) ForeverNow(interval time.Duration, f func()) int64 {
	s.executeAsync(f)
	return s.schedule(interval, true, f)
}

// Cancel 取消指定任务
func (s *scheduler) Cancel(taskID int64) {
	s.removeTask(taskID)
}

// CancelAll 取消所有任务
func (s *scheduler) CancelAll() {
	s.tasks.Range(func(key, _ any) bool {
		s.removeTask(key.(int64))
		return true
	})
}

func (s *scheduler) removeTask(taskID int64) {
	val, ok := s.tasks.Load(taskID)
	if !ok {
		return
	}
	entry := val.(*taskEntry)

	// 标记为取消
	if !entry.cancelled.CompareAndSwap(false, true) {
		return
	}

	// 停止 timer
	if entry.timer != nil {
		entry.timer.Stop()
		entry.timer = nil
	}

	// 等待执行完成（最多 100ms）
	for i := 0; i < 10 && entry.executing.Load(); i++ {
		time.Sleep(10 * time.Millisecond)
	}

	s.tasks.Delete(taskID)
	entry.task = nil
}

// Stop 停止调度器，等待正在执行任务完成
func (s *scheduler) Stop() {
	s.once.Do(func() {
		s.shutdown.Store(true)
		s.cancel()
		s.CancelAll()

		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()

		timeout := s.stopTimeout
		if timeout <= 0 {
			timeout = 3 * time.Second
		}

		select {
		case <-done:
			log.Info("[scheduler] stopped gracefully")
		case <-time.After(timeout):
			log.Warnf("[scheduler] shutdown timed out after %v, some tasks may still be running", timeout)
		}
	})
}

// schedule 注册任务
func (s *scheduler) schedule(delay time.Duration, repeated bool, f func()) int64 {
	if s.shutdown.Load() || s.ctx.Err() != nil {
		log.Warn("scheduler is shut down; task rejected")
		return -1
	}

	taskID := s.nextID.Add(1)
	entry := &taskEntry{repeated: repeated, task: f}
	s.tasks.Store(taskID, entry) // 先存储到 map，防止 timer 先触发 wrapped 导致 removeTask 找不到
	startAt := time.Now()

	wrapped := func() {
		wrappedAt := time.Now()

		// 检查取消状态
		if entry.cancelled.Load() {
			return
		}

		// 仅对一次性任务防止重复执行
		if !repeated && !entry.executing.CompareAndSwap(false, true) {
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

	return taskID
}

func (s *scheduler) executeAsync(f func()) {
	run := func() {
		defer RecoverFromError(nil)
		f()
	}
	if s.executor != nil {
		s.executor.Post(run)
	} else {
		go run()
	}
}

// log debug
func (s *scheduler) lazy(taskID int64, delay time.Duration, startAt, execAt, wrappedAt time.Time) {
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
