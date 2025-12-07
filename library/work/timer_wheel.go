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

// wheelPreciseEvery 精准周期定时器，防止时间漂移
type wheelPreciseEvery struct {
	Interval time.Duration
	last     atomic.Value // time.Time，上次执行时间
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

func WithWheelContext(ctx context.Context) WheelSchedulerOption {
	return func(s *wheelScheduler) { s.ctx = ctx }
}

func WithWheelExecutor(exec IExecutor) WheelSchedulerOption {
	return func(s *wheelScheduler) { s.baseScheduler.executor = exec }
}

func WithStopTimeout(timeout time.Duration) WheelSchedulerOption {
	return func(s *wheelScheduler) {
		if timeout > 0 {
			s.stopTimeout = timeout
		}
	}
}

// wheelScheduler 基于时间轮的定时任务调度器
type wheelScheduler struct {
	baseScheduler                          // 嵌入通用基础功能
	tick          time.Duration            // 时间轮精度
	wheelSize     int64                    // 时间轮槽位数
	tw            *timingwheel.TimingWheel // 时间轮实例
	stopTimeout   time.Duration            // Stop 超时时间
	tasks         sync.Map                 // 任务映射，key为任务ID，value为*wheelTaskEntry
	nextID        atomic.Int64             // 任务ID生成器
	shutdown      atomic.Bool              // 调度器是否已关闭
	ctx           context.Context          // 上下文，用于控制调度器生命周期
	cancel        context.CancelFunc       // 取消函数
	wg            sync.WaitGroup           // 等待所有任务完成
	once          sync.Once                // 确保 Stop 只执行一次
}

// wheelTaskEntry 时间轮调度器任务项
type wheelTaskEntry struct {
	timer     *timingwheel.Timer // 时间轮定时器
	cancelled atomic.Bool        // 是否已取消
	repeated  bool               // 是否周期任务
	executing atomic.Bool        // 是否正在执行
	task      func()             // 任务函数
}

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

	if s.baseScheduler.executor == nil {
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
		Capacity: 0,
		Len:      s.Len(),
		Running:  s.Running(),
	}
}

func (s *wheelScheduler) Once(delay time.Duration, f func()) int64 {
	return s.schedule(delay, false, f)
}

func (s *wheelScheduler) Forever(interval time.Duration, f func()) int64 {
	return s.schedule(interval, true, f)
}

func (s *wheelScheduler) ForeverNow(interval time.Duration, f func()) int64 {
	s.baseScheduler.executeAsync(f)
	return s.schedule(interval, true, f)
}

func (s *wheelScheduler) Cancel(taskID int64) {
	s.removeTask(taskID)
}

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

	if !entry.cancelled.CompareAndSwap(false, true) {
		return
	}

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

func (s *wheelScheduler) Stop() {
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
			log.Info("[wheelScheduler] stopped gracefully")
		case <-time.After(timeout):
			log.Warnf("[wheelScheduler] shutdown timed out after %v", timeout)
		}
	})
}

func (s *wheelScheduler) schedule(delay time.Duration, repeated bool, f func()) int64 {
	if s.shutdown.Load() || s.ctx.Err() != nil {
		log.Warn("[wheelScheduler] is shut down; task rejected")
		return -1
	}

	taskID := s.nextID.Add(1)
	entry := &wheelTaskEntry{repeated: repeated, task: f}
	s.tasks.Store(taskID, entry)
	startAt := time.Now()

	wrapped := func() {
		wrappedAt := time.Now()
		if entry.cancelled.Load() {
			return
		}
		if !repeated && !entry.executing.CompareAndSwap(false, true) {
			return
		}
		s.incrementRunning()
		s.wg.Add(1)

		s.baseScheduler.executeAsync(func() {
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

func (s *wheelScheduler) lazy(taskID int64, delay time.Duration, startAt, execAt, wrappedAt time.Time) {
	now := time.Now()
	lazy := now.Sub(startAt)
	latency := lazy - delay

	if latency >= s.tick {
		exec, wrapped := now.Sub(execAt), now.Sub(wrappedAt)
		log.Errorf("[wheelScheduler] taskID=%d delay=%v precision=%v lazy=%v latency=%v exec=%+v wrap=%+v",
			taskID, delay, s.tick, lazy, latency, exec, wrapped-exec)
	}
}
