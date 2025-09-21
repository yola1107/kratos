package work

import (
	"container/heap"
	"context"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yola1107/kratos/v2/log"
)

const (
	defaultTickPrecision = 100 * time.Millisecond // 默认调度循环精度
	maxIntervalJumps     = 10000                  // 防止周期任务因为滞后而无限补跑
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
	Capacity int   // 堆底层切片容量
	Len      int   // 当前注册任务数量
	Running  int32 // 当前执行中的任务数量
}

// taskEntry 任务结构体
type taskEntry struct {
	id        int64         // 任务ID
	execAt    time.Time     // 下一次执行时间
	interval  time.Duration // 周期任务间隔
	repeated  bool          // 是否周期任务
	cancelled atomic.Bool   // 是否已取消
	task      func()        // 任务函数
	index     int           // 堆索引，用于从堆中删除
}

// 小顶堆，用于按任务执行时间排序
type taskHeap []*taskEntry

func (h taskHeap) Len() int           { return len(h) }
func (h taskHeap) Less(i, j int) bool { return h[i].execAt.Before(h[j].execAt) }
func (h taskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *taskHeap) Push(x any) {
	entry := x.(*taskEntry)
	entry.index = len(*h)
	*h = append(*h, entry)
}
func (h *taskHeap) Pop() any {
	old := *h
	n := len(old)
	entry := old[n-1]
	entry.index = -1
	*h = old[0 : n-1]
	return entry
}

// SchedulerOption 调度器选项
type SchedulerOption func(*Scheduler)

func WithExecutor(exec ITaskExecutor) SchedulerOption {
	return func(s *Scheduler) { s.executor = exec }
}

func WithContext(ctx context.Context) SchedulerOption {
	return func(s *Scheduler) { s.ctx = ctx }
}

// Scheduler 定时任务调度器，基于最小堆实现
type Scheduler struct {
	executor ITaskExecutor // 可选自定义执行器
	mu       sync.Mutex
	tasks    map[int64]*taskEntry // 所有任务映射
	h        taskHeap             // 小顶堆
	nextID   atomic.Int64         // 任务ID递增
	running  atomic.Int32         // 当前执行任务数
	shutdown atomic.Bool          // 调度器是否关闭
	timer    *time.Timer          //
	ctx      context.Context      //
	cancel   context.CancelFunc   //
	wg       sync.WaitGroup       //
	once     sync.Once            //
	wakeup   chan struct{}        // 唤醒循环的新任务信号
}

// NewTaskScheduler 创建调度器实例
func NewTaskScheduler(opts ...SchedulerOption) ITaskScheduler {
	s := &Scheduler{
		tasks:  make(map[int64]*taskEntry),
		wakeup: make(chan struct{}, 1),
		ctx:    context.Background(),
	}
	for _, opt := range opts {
		opt(s)
	}

	s.ctx, s.cancel = context.WithCancel(s.ctx)
	s.timer = time.NewTimer(time.Hour)
	if !s.timer.Stop() {
		select {
		case <-s.timer.C:
		default:
		}
	}

	go s.loop() // 启动调度主循环
	return s
}

// loop 主循环，按堆顶任务时间执行任务
func (s *Scheduler) loop() {
	defer RecoverFromError(func(e any) { go s.loop() }) // 异常恢复并重启循环

	for {
		s.mu.Lock()
		var expired []*taskEntry
		now := time.Now()

		// 批量弹出已到期任务
		for len(s.h) > 0 && !s.h[0].execAt.After(now) {
			entry := heap.Pop(&s.h).(*taskEntry)
			if entry.cancelled.Load() {
				delete(s.tasks, entry.id)
				entry.task = nil // 已取消任务，帮助 GC 释放闭包引用
				continue
			}
			delete(s.tasks, entry.id)
			expired = append(expired, entry)
		}
		s.mu.Unlock()

		// 异步执行到期任务
		for _, entry := range expired {
			s.running.Add(1)
			s.wg.Add(1)
			t := entry
			s.executeAsync(func() {
				defer func() {
					RecoverFromError(nil)
					s.wg.Done()
					s.running.Add(-1)
					// 一次性任务执行完，释放闭包引用
					if !t.repeated {
						t.task = nil // 执行完任务后帮助 GC 释放闭包引用
					}
				}()
				if t.cancelled.Load() {
					return
				}
				t.task()
			})

			// 周期任务，重新入堆
			if t.repeated && !t.cancelled.Load() {
				t.execAt = t.execAt.Add(t.interval) // 精准周期
				s.mu.Lock()
				s.tasks[t.id] = t
				heap.Push(&s.h, t)
				s.signalWakeup() // 唤醒 loop 重新计算等待时间
				s.mu.Unlock()
			}
		}

		// 计算下一次等待时间
		s.mu.Lock()
		var wait time.Duration
		if len(s.h) == 0 {
			wait = time.Hour // 没有任务时长时间等待
		} else {
			wait = s.h[0].execAt.Sub(now)
			if wait < 0 {
				wait = 0
			}
		}
		if !s.timer.Stop() {
			select {
			case <-s.timer.C:
			default:
			}
		}
		s.timer.Reset(wait)
		s.mu.Unlock()

		// 阻塞等待下一次任务或唤醒信号
		select {
		case <-s.timer.C:
		case <-s.wakeup:
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Scheduler) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.tasks)
}

func (s *Scheduler) Running() int32 {
	return s.running.Load()
}

func (s *Scheduler) Monitor() Monitor {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Monitor{
		Capacity: cap(s.h),
		Len:      len(s.tasks),
		Running:  s.Running(),
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.tasks[taskID]; ok {
		entry.cancelled.Store(true)
		delete(s.tasks, taskID)
		if entry.index >= 0 && entry.index < len(s.h) {
			heap.Remove(&s.h, entry.index)
		}
		entry.task = nil // 帮助 GC 释放闭包引用
		s.signalWakeup() // 唤醒 loop，避免阻塞等待已取消任务
	}
}

// CancelAll 取消所有任务
func (s *Scheduler) CancelAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range s.tasks {
		entry.cancelled.Store(true)
		entry.task = nil // 帮助 GC 释放闭包引用
	}
	s.tasks = make(map[int64]*taskEntry)
	s.h = taskHeap{}
	s.signalWakeup()
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
	entry := &taskEntry{
		id:       taskID,
		execAt:   time.Now().Add(delay),
		interval: delay,
		repeated: repeated,
		task:     f,
	}

	s.mu.Lock()
	s.tasks[taskID] = entry
	heap.Push(&s.h, entry)
	s.signalWakeup() // 唤醒 loop 重新计算等待时间
	s.mu.Unlock()
	return taskID
}

// executeAsync 执行任务函数（支持自定义执行器）
func (s *Scheduler) executeAsync(f func()) {
	wrapped := func() {
		defer RecoverFromError(nil)
		f()
	}
	if s.executor != nil {
		s.executor.Post(wrapped)
	} else {
		go wrapped()
	}
}

// signalWakeup 发送唤醒信号
func (s *Scheduler) signalWakeup() {
	select {
	case s.wakeup <- struct{}{}:
	default: // 避免阻塞
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
