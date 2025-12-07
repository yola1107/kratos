package work

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RussellLuo/timingwheel"

	"github.com/yola1107/kratos/v2/log"
)

const (
	defaultWheelTickPrecision = 500 * time.Millisecond // 时间轮调度器默认精度
	defaultWheelSize          = 128                    // 时间轮默认槽位数
)

// wheelPreciseEvery 实现精准的周期性定时器，防止时间漂移
type wheelPreciseEvery struct {
	Interval time.Duration
	last     atomic.Value // time.Time
}

func (p *wheelPreciseEvery) Next(t time.Time) time.Time {
	last, _ := p.last.Load().(time.Time)
	if last.IsZero() {
		last = t
	}
	steps := 0
	next := last.Add(p.Interval)
	for !next.After(t) {
		next = next.Add(p.Interval)
		if steps++; steps > maxIntervalJumps {
			log.Warnf("[wheelPreciseEvery] skipped too many steps: %d", steps)
			break
		}
	}
	p.last.Store(next)
	return next
}

// WheelSchedulerOption 调度器选项
type WheelSchedulerOption func(*wheelScheduler)

func WithTick(d time.Duration) WheelSchedulerOption {
	return func(s *wheelScheduler) {
		if d > 0 {
			s.tick = d
		} else {
			log.Warnf("Invalid tick %v, using default %v", d, defaultWheelTickPrecision)
		}
	}
}

func WithWheelSize(size int64) WheelSchedulerOption {
	return func(s *wheelScheduler) {
		if size > 0 {
			s.wheelSize = size
		} else {
			log.Warnf("Invalid wheelSize %d, using default %d", size, defaultWheelSize)
		}
	}
}

func WithContext(ctx context.Context) WheelSchedulerOption {
	return func(s *wheelScheduler) { s.ctx = ctx }
}

func WithExecutor(exec IExecutor) WheelSchedulerOption {
	return func(s *wheelScheduler) { s.executor = exec }
}

func WithStopTimeout(timeout time.Duration) WheelSchedulerOption {
	return func(s *wheelScheduler) {
		if timeout > 0 {
			s.stopTimeout = timeout
		}
	}
}

// wheelScheduler 定时任务调度器，基于时间轮实现
type wheelScheduler struct {
	baseScheduler                          // 嵌入通用基础功能
	tick          time.Duration            // 精度
	wheelSize     int64                    // 槽位
	tw            *timingwheel.TimingWheel // 时间轮
	stopTimeout   time.Duration            // Stop 超时时间
	tasks         sync.Map                 // map[int64]*wheelTaskEntry
	nextID        atomic.Int64             // 任务ID递增
	shutdown      atomic.Bool              // 是否关闭
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	once          sync.Once
}

type wheelTaskEntry struct {
	timer     *timingwheel.Timer
	cancelled atomic.Bool
	repeated  bool
	executing atomic.Bool
	task      func()
}

// NewWheelScheduler 创建时间轮调度器实例
func NewWheelScheduler(opts ...WheelSchedulerOption) Scheduler {
	s := &wheelScheduler{
		tick:        defaultWheelTickPrecision,
		wheelSize:   defaultWheelSize,
		ctx:         context.Background(),
		stopTimeout: 3 * time.Second, // 默认超时 3 秒
	}
	for _, opt := range opts {
		opt(s)
	}

	if s.executor == nil {
		log.Warn("[wheelScheduler] No executor provided, tasks will run in unlimited goroutines")
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

func (s *wheelScheduler) Len() int {
	count := 0
	s.tasks.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

func (s *wheelScheduler) Running() int32 {
	return s.getRunning()
}

func (s *wheelScheduler) Monitor() Monitor {
	return Monitor{
		Capacity: 0, // 时间轮调度器不提供容量信息
		Len:      s.Len(),
		Running:  s.Running(),
	}
}

// Once 注册一次性任务
func (s *wheelScheduler) Once(delay time.Duration, f func()) int64 {
	return s.schedule(delay, false, f)
}

// Forever 注册周期任务
func (s *wheelScheduler) Forever(interval time.Duration, f func()) int64 {
	return s.schedule(interval, true, f)
}

// ForeverNow 注册周期任务并立即执行一次
func (s *wheelScheduler) ForeverNow(interval time.Duration, f func()) int64 {
	s.executeAsync(f)
	return s.schedule(interval, true, f)
}

// Cancel 取消指定任务
func (s *wheelScheduler) Cancel(taskID int64) {
	s.removeTask(taskID)
}

// CancelAll 取消所有任务
func (s *wheelScheduler) CancelAll() {
	s.tasks.Range(func(key, _ any) bool {
		s.removeTask(key.(int64))
		return true
	})
}

func (s *wheelScheduler) removeTask(taskID int64) {
	val, ok := s.tasks.Load(taskID)
	if !ok {
		return
	}
	entry := val.(*wheelTaskEntry)

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
func (s *wheelScheduler) Stop() {
	s.once.Do(func() {
		s.shutdown.Store(true)
		s.cancel()
		s.CancelAll()

		// 等待任务完成
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
			log.Info("[wheelScheduler] stopped gracefully")
		case <-time.After(timeout):
			log.Warnf("[wheelScheduler] shutdown timed out after %v, some tasks may still be running", timeout)
		}
	})
}

// schedule 注册任务
func (s *wheelScheduler) schedule(delay time.Duration, repeated bool, f func()) int64 {
	if s.shutdown.Load() || s.ctx.Err() != nil {
		log.Warn("wheelScheduler is shut down; task rejected")
		return -1
	}

	taskID := s.nextID.Add(1)
	entry := &wheelTaskEntry{repeated: repeated, task: f}
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
		s.incrementRunning()
		s.wg.Add(1)

		s.executeAsync(func() {
			execAt := time.Now()
			defer func() {
				RecoverFromError(nil)
				s.wg.Done()
				s.decrementRunning()
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
		entry.timer = s.tw.ScheduleFunc(&wheelPreciseEvery{Interval: delay}, wrapped)
	} else {
		entry.timer = s.tw.AfterFunc(delay, wrapped)
	}

	return taskID
}

func (s *wheelScheduler) executeAsync(f func()) {
	s.baseScheduler.executeAsync(f)
}

// log debug
func (s *wheelScheduler) lazy(taskID int64, delay time.Duration, startAt, execAt, wrappedAt time.Time) {
	now := time.Now()
	lazy := now.Sub(startAt)
	latency := lazy - delay

	if latency >= s.tick {
		exec, wrapped := now.Sub(execAt), now.Sub(wrappedAt)
		log.Errorf("[wheelScheduler] taskID=%d delay=%v precision=%v lazy=%v latency=%v exec=%+v wrap=%+v",
			taskID, delay, s.tick, lazy, latency, exec, wrapped-exec,
		)
	}
}
