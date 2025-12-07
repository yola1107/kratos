package work

import (
	"container/heap"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yola1107/kratos/v2/log"
)

const (
	defaultHeapTickPrecision = 100 * time.Millisecond // 默认调度循环精度（堆实现）
)

// heapTaskEntry 堆调度器任务项
type heapTaskEntry struct {
	id        int64         // 任务ID
	execAt    time.Time     // 下一次执行时间
	interval  time.Duration // 周期任务间隔
	repeated  bool          // 是否周期任务
	cancelled atomic.Bool   // 是否已取消
	task      func()        // 任务函数
	index     int           // 堆索引，用于从堆中删除
}

// taskQueue 任务队列，使用最小堆管理定时任务
type taskQueue struct {
	mu    sync.Mutex               // 保护并发访问
	heap  []*heapTaskEntry         // 最小堆，按执行时间排序
	tasks map[int64]*heapTaskEntry // 任务ID到任务的映射，用于快速查找
}

func newTaskQueue() *taskQueue {
	return &taskQueue{
		heap:  make([]*heapTaskEntry, 0),
		tasks: make(map[int64]*heapTaskEntry),
	}
}

// 实现 heap.Interface
func (q *taskQueue) Len() int { return len(q.heap) }
func (q *taskQueue) Less(i, j int) bool {
	return q.heap[i].execAt.Before(q.heap[j].execAt)
}
func (q *taskQueue) Swap(i, j int) {
	q.heap[i], q.heap[j] = q.heap[j], q.heap[i]
	q.heap[i].index = i
	q.heap[j].index = j
}
func (q *taskQueue) Push(x any) {
	t := x.(*heapTaskEntry)
	t.index = len(q.heap)
	q.heap = append(q.heap, t)
}
func (q *taskQueue) Pop() any {
	n := len(q.heap)
	t := q.heap[n-1]
	t.index = -1
	q.heap = q.heap[:n-1]
	return t
}

func (q *taskQueue) AddTask(t *heapTaskEntry) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks[t.id] = t
	needWake := len(q.heap) == 0 || t.execAt.Before(q.heap[0].execAt)
	heap.Push(q, t)
	return needWake
}

func (q *taskQueue) PopExpired(now time.Time) []*heapTaskEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	var expired []*heapTaskEntry
	for len(q.heap) > 0 && !q.heap[0].execAt.After(now) {
		t := heap.Pop(q).(*heapTaskEntry)
		delete(q.tasks, t.id)
		if !t.cancelled.Load() {
			expired = append(expired, t)
		}
	}
	return expired
}

func (q *taskQueue) RemoveTask(taskID int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	t, ok := q.tasks[taskID]
	if !ok {
		return
	}
	// 只标记为取消，不立即清空 task，避免正在执行的任务 panic
	t.cancelled.Store(true)
	delete(q.tasks, taskID)
	if t.index >= 0 && t.index < len(q.heap) {
		heap.Remove(q, t.index)
	}
}

func (q *taskQueue) TaskCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.tasks)
}

func (q *taskQueue) NextExecDuration(now time.Time) time.Duration {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.heap) == 0 {
		return time.Hour
	}
	if d := q.heap[0].execAt.Sub(now); d > 0 {
		return d
	}
	return 0
}

type HeapSchedulerOption func(*heapScheduler)

func WithHeapExecutor(exec IExecutor) HeapSchedulerOption {
	return func(s *heapScheduler) { s.baseScheduler.executor = exec }
}

func WithHeapContext(ctx context.Context) HeapSchedulerOption {
	return func(s *heapScheduler) { s.ctx = ctx }
}

// heapScheduler 基于最小堆的定时任务调度器
type heapScheduler struct {
	baseScheduler                    // 嵌入通用基础功能
	queue         *taskQueue         // 任务队列
	nextID        atomic.Int64       // 任务ID生成器
	shutdown      atomic.Bool        // 调度器是否已关闭
	timer         *time.Timer        // 定时器，用于等待下一个任务
	ctx           context.Context    // 上下文，用于控制调度器生命周期
	cancel        context.CancelFunc // 取消函数
	wg            sync.WaitGroup     // 等待所有任务完成
	wakeup        chan struct{}      // 唤醒信号通道，用于新任务唤醒循环
}

func NewHeapScheduler(opts ...HeapSchedulerOption) Scheduler {
	s := &heapScheduler{
		queue:  newTaskQueue(),
		wakeup: make(chan struct{}, 1),
		ctx:    context.Background(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.ctx, s.cancel = context.WithCancel(s.ctx)
	s.timer = time.NewTimer(time.Hour)
	s.timer.Stop() // 立即停止，等待首次 Reset
	go s.loop()
	return s
}

func (s *heapScheduler) loop() {
	defer RecoverFromError(func(e any) { go s.loop() })
	for {
		expired := s.queue.PopExpired(time.Now())
		for _, t := range expired {
			// 先保存任务函数，避免在异步执行时访问可能被修改的字段
			taskFn := t.task
			if taskFn == nil {
				continue
			}

			s.incrementRunning()
			s.wg.Add(1)
			task := t
			s.baseScheduler.executeAsync(func() {
				defer func() {
					RecoverFromError(nil)
					s.decrementRunning()
					s.wg.Done()
				}()
				// 执行前再次检查是否已取消
				if !task.cancelled.Load() {
					taskFn()
				}
			})

			// 周期任务重新入队（需要加锁保护，避免与 CancelAll 竞争）
			if task.repeated && !task.cancelled.Load() {
				task.execAt = task.execAt.Add(task.interval)
				// 重新入队前再次检查，避免在计算新时间时被取消
				if !task.cancelled.Load() && s.queue.AddTask(task) {
					s.wakeupLoop()
				}
			}
		}

		s.resetTimer(s.queue.NextExecDuration(time.Now()))

		select {
		case <-s.timer.C:
		case <-s.wakeup:
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *heapScheduler) Len() int       { return s.queue.TaskCount() }
func (s *heapScheduler) Running() int32 { return s.getRunning() }
func (s *heapScheduler) Monitor() Monitor {
	s.queue.mu.Lock()
	defer s.queue.mu.Unlock()
	return Monitor{
		Capacity: cap(s.queue.heap),
		Len:      len(s.queue.heap),
		Running:  s.Running(),
	}
}

func (s *heapScheduler) Once(delay time.Duration, f func()) int64 {
	return s.schedule(delay, false, f)
}

func (s *heapScheduler) Forever(interval time.Duration, f func()) int64 {
	return s.schedule(interval, true, f)
}

func (s *heapScheduler) ForeverNow(interval time.Duration, f func()) int64 {
	s.baseScheduler.executeAsync(f)
	return s.schedule(interval, true, f)
}

func (s *heapScheduler) Cancel(taskID int64) {
	s.queue.RemoveTask(taskID)
	s.wakeupLoop()
}

func (s *heapScheduler) CancelAll() {
	s.queue.mu.Lock()
	defer s.queue.mu.Unlock()
	// 先标记所有任务为取消，不立即清空 task，避免正在执行的任务 panic
	for _, t := range s.queue.tasks {
		t.cancelled.Store(true)
	}
	s.queue.heap = []*heapTaskEntry{}
	s.queue.tasks = make(map[int64]*heapTaskEntry)
	s.wakeupLoop()
}

func (s *heapScheduler) Stop() {
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
		log.Warn("[heapScheduler] shutdown timed out")
	}
}

func (s *heapScheduler) schedule(delay time.Duration, repeated bool, f func()) int64 {
	if s.shutdown.Load() || s.ctx.Err() != nil {
		log.Warn("[heapScheduler] is shut down; task rejected")
		return -1
	}
	taskID := s.nextID.Add(1)
	t := &heapTaskEntry{
		id:       taskID,
		execAt:   time.Now().Add(delay),
		interval: delay,
		repeated: repeated,
		task:     f,
	}
	if s.queue.AddTask(t) {
		s.wakeupLoop()
	}
	return taskID
}

func (s *heapScheduler) wakeupLoop() {
	select {
	case s.wakeup <- struct{}{}:
	default:
	}
}

func (s *heapScheduler) resetTimer(d time.Duration) {
	if !s.timer.Stop() {
		select {
		case <-s.timer.C:
		default:
		}
	}
	s.timer.Reset(d)
}
