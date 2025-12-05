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

// taskQueue 小顶堆heap + map
type taskQueue struct {
	mu    sync.Mutex
	heap  []*taskEntry         // 最小堆，根据 execAt 排序
	tasks map[int64]*taskEntry // 任务映射
}

// newTaskQueue 创建优先任务队列
func newTaskQueue() *taskQueue {
	return &taskQueue{
		heap:  make([]*taskEntry, 0),
		tasks: make(map[int64]*taskEntry),
	}
}

// Len heap 接口实现
func (q *taskQueue) Len() int { return len(q.heap) }
func (q *taskQueue) Less(i, j int) bool {
	return q.heap[i].execAt.Before(q.heap[j].execAt)
}
func (q *taskQueue) Swap(i, j int) {
	q.heap[i], q.heap[j] = q.heap[j], q.heap[i]
	q.heap[i].index = i
	q.heap[j].index = j
}

// Push heap 接口实现
func (q *taskQueue) Push(x any) {
	t := x.(*taskEntry)
	t.index = len(q.heap)
	q.heap = append(q.heap, t)
}

// Pop heap 接口实现
func (q *taskQueue) Pop() any {
	n := len(q.heap)
	t := q.heap[n-1]
	t.index = -1
	q.heap = q.heap[:n-1]
	return t
}

// AddTask 新增任务并入堆，返回是否需要唤醒循环
func (q *taskQueue) AddTask(t *taskEntry) (needWake bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks[t.id] = t

	// 判断是否比堆顶任务更早
	earliest := time.Time{}
	if len(q.heap) > 0 {
		earliest = q.heap[0].execAt
	}

	heap.Push(q, t)

	// 只有新任务更早或堆为空才唤醒
	if earliest.IsZero() || t.execAt.Before(earliest) {
		return true
	}
	return false
}

// PopExpired 弹出所有到期任务
func (q *taskQueue) PopExpired(now time.Time) []*taskEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	var expired []*taskEntry
	for len(q.heap) > 0 && !q.heap[0].execAt.After(now) {
		t := heap.Pop(q).(*taskEntry)
		delete(q.tasks, t.id)
		if !t.cancelled.Load() {
			expired = append(expired, t)
		}
	}
	return expired
}

// RemoveTask 取消任务
func (q *taskQueue) RemoveTask(taskID int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	t, ok := q.tasks[taskID]
	if !ok {
		return
	}
	t.cancelled.Store(true)
	t.task = nil // 释放任务函数引用，避免内存泄漏
	delete(q.tasks, taskID)
	if t.index >= 0 && t.index < len(q.heap) {
		heap.Remove(q, t.index)
	}
}

// TaskCount 返回当前任务数量
func (q *taskQueue) TaskCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.tasks)
}

// NextExecDuration 获取下一个任务间隔
func (q *taskQueue) NextExecDuration(now time.Time) time.Duration {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.heap) == 0 {
		return time.Hour
	}
	d := q.heap[0].execAt.Sub(now)
	if d < 0 {
		return 0
	}
	return d
}

// SchedulerOption 调度器选项
type SchedulerOption func(*Scheduler)

// WithExecutor 设置自定义执行器
func WithExecutor(exec ITaskExecutor) SchedulerOption {
	return func(s *Scheduler) { s.executor = exec }
}

// WithContext 设置上下文
func WithContext(ctx context.Context) SchedulerOption {
	return func(s *Scheduler) { s.ctx = ctx }
}

// Scheduler 定时任务调度器
type Scheduler struct {
	executor ITaskExecutor      // 可选自定义执行器
	queue    *taskQueue         // 小顶堆
	nextID   atomic.Int64       // 任务ID递增
	running  atomic.Int32       // 当前执行任务数
	shutdown atomic.Bool        // 调度器是否关闭
	timer    *time.Timer        // 复用timer
	ctx      context.Context    //
	cancel   context.CancelFunc //
	wg       sync.WaitGroup     //
	wakeup   chan struct{}      // 唤醒循环的新任务信号
}

// NewTaskScheduler 创建调度器实例
func NewTaskScheduler(opts ...SchedulerOption) ITaskScheduler {
	s := &Scheduler{
		queue:  newTaskQueue(),
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
	go s.loop()
	return s
}

// loop 主循环，按堆顶任务时间执行任务
func (s *Scheduler) loop() {
	defer RecoverFromError(func(e any) { go s.loop() })
	for {
		now := time.Now()
		expired := s.queue.PopExpired(now)

		// 异步执行到期任务
		for _, t := range expired {
			s.running.Add(1)
			s.wg.Add(1)
			task := t
			s.executeAsync(func() {
				defer func() {
					RecoverFromError(nil)
					s.running.Add(-1)
					s.wg.Done()
				}()
				task.task()
			})

			// 周期任务重新入队
			if task.repeated && !task.cancelled.Load() {
				task.execAt = task.execAt.Add(task.interval)
				if s.queue.AddTask(task) {
					s.signalWakeup() // 仅在新任务更早时唤醒
				}
			}
		}

		wait := s.queue.NextExecDuration(time.Now())
		if !s.timer.Stop() {
			select {
			case <-s.timer.C:
			default:
			}
		}
		s.timer.Reset(wait)

		select {
		case <-s.timer.C:
		case <-s.wakeup:
		case <-s.ctx.Done():
			return
		}
	}
}

// Len 当前任务数量
func (s *Scheduler) Len() int { return s.queue.TaskCount() }

// Running 当前执行任务数量
func (s *Scheduler) Running() int32 { return s.running.Load() }

// Monitor 获取任务池状态
func (s *Scheduler) Monitor() Monitor {
	s.queue.mu.Lock()
	defer s.queue.mu.Unlock()
	return Monitor{
		Capacity: cap(s.queue.heap),
		Len:      len(s.queue.heap),
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

// Cancel 取消任务
func (s *Scheduler) Cancel(taskID int64) {
	s.queue.RemoveTask(taskID)
	s.signalWakeup()
}

// CancelAll 取消所有任务
func (s *Scheduler) CancelAll() {
	s.queue.mu.Lock()
	defer s.queue.mu.Unlock()
	for _, t := range s.queue.tasks {
		t.cancelled.Store(true)
		t.task = nil // 释放函数引用
	}
	s.queue.heap = []*taskEntry{}
	s.queue.tasks = make(map[int64]*taskEntry)
	s.signalWakeup()
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	if !s.shutdown.CompareAndSwap(false, true) {
		return
	}
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
}

// schedule 注册任务
func (s *Scheduler) schedule(delay time.Duration, repeated bool, f func()) int64 {
	if s.shutdown.Load() || s.ctx.Err() != nil {
		log.Warn("scheduler is shut down; task rejected")
		return -1
	}
	taskID := s.nextID.Add(1)
	t := &taskEntry{
		id:       taskID,
		execAt:   time.Now().Add(delay),
		interval: delay,
		repeated: repeated,
		task:     f,
	}
	if s.queue.AddTask(t) {
		s.signalWakeup() // 仅必要时唤醒
	}
	return taskID
}

// executeAsync 异步执行任务
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
	default:
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
