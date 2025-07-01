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
	defaultTickPrecision = 100 * time.Millisecond
	defaultWheelSize     = int64(512)
)

// TaskType 定义更明确的任务类型
type TaskType int

const (
	TaskOnce       TaskType = iota // 单次任务
	TaskFixedRate                  // 固定速率（基于起始时间）
	TaskFixedDelay                 // 固定延迟（基于结束时间）
)

// ITaskScheduler 调度器接口
type ITaskScheduler interface {
	Len() int
	Running() int32
	Once(delay time.Duration, f func()) int64
	Schedule(at time.Time, f func()) int64
	FixedRate(delay, interval time.Duration, f func()) int64
	FixedDelay(delay, interval time.Duration, f func()) int64
	Forever(interval time.Duration, f func()) int64
	ForeverNow(interval time.Duration, f func()) int64
	Cancel(taskID int64)
	CancelAll()
	Stop()
}

// ITaskExecutor 任务执行器接口
type ITaskExecutor interface {
	Post(job func())
}

type SchedulerOption func(*taskScheduler)

func WithContext(ctx context.Context) SchedulerOption {
	return func(cfg *taskScheduler) {
		cfg.ctx = ctx
	}
}

func WithExecutor(exec ITaskExecutor) SchedulerOption {
	return func(cfg *taskScheduler) {
		cfg.executor = exec
	}
}

// taskScheduler 实现
type taskScheduler struct {
	tasks     sync.Map
	seq       atomic.Int64
	count     atomic.Int32
	executor  ITaskExecutor
	ctx       context.Context
	cancel    context.CancelFunc
	taskQueue *taskPriorityQueue
	wg        sync.WaitGroup
	stopOnce  sync.Once
	queueMu   sync.Mutex
	running   atomic.Bool
	wakeCh    chan struct{} // 用于唤醒调度循环
}

// scheduledTask 表示调度任务
type scheduledTask struct {
	id       int64
	taskType TaskType
	execute  func()
	nextRun  time.Time
	interval time.Duration
	active   atomic.Bool
}

// taskPriorityQueue 优先队列
type taskPriorityQueue []*scheduledTask

func (pq taskPriorityQueue) Len() int            { return len(pq) }
func (pq taskPriorityQueue) Less(i, j int) bool  { return pq[i].nextRun.Before(pq[j].nextRun) }
func (pq taskPriorityQueue) Swap(i, j int)       { pq[i], pq[j] = pq[j], pq[i] }
func (pq *taskPriorityQueue) Push(x interface{}) { *pq = append(*pq, x.(*scheduledTask)) }
func (pq *taskPriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	task := old[n-1]
	*pq = old[0 : n-1]
	return task
}

// NewTaskScheduler 创建调度器
func NewTaskScheduler(opts ...SchedulerOption) ITaskScheduler {
	s := &taskScheduler{
		executor:  nil,
		ctx:       context.Background(),
		taskQueue: &taskPriorityQueue{},
		wakeCh:    make(chan struct{}, 1),
	}
	for _, opt := range opts {
		opt(s)
	}

	ctx, cancel := context.WithCancel(s.ctx)
	s.ctx = ctx
	s.cancel = cancel

	heap.Init(s.taskQueue)
	s.start()
	return s
}

// Start 启动调度器
func (t *taskScheduler) start() {
	if t.running.CompareAndSwap(false, true) {
		go t.schedulerLoop()
	}
}

// 调度器主循环 - 使用单一定时器优化调度精度
func (t *taskScheduler) schedulerLoop() {
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()

	for t.running.Load() {
		var nextWake *time.Time
		var waitDuration time.Duration

		// 获取下一个任务的时间
		t.queueMu.Lock()
		if t.taskQueue.Len() > 0 {
			next := (*t.taskQueue)[0]
			nextWake = &next.nextRun
		}
		t.queueMu.Unlock()

		if nextWake != nil {
			waitDuration = time.Until(*nextWake)
			if waitDuration <= 0 {
				t.processDueTasks()
				continue
			}
		} else {
			// 没有任务时等待唤醒
			select {
			case <-t.wakeCh:
				continue
			case <-t.ctx.Done():
				return
			}
		}

		// 重置定时器
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(waitDuration)

		select {
		case <-timer.C:
			t.processDueTasks()
		case <-t.wakeCh:
			// 新任务唤醒，重新检查队列
		case <-t.ctx.Done():
			return
		}
	}
}

// 处理到期任务
func (t *taskScheduler) processDueTasks() {
	t.queueMu.Lock()
	defer t.queueMu.Unlock()

	now := time.Now()
	for t.taskQueue.Len() > 0 {
		task := (*t.taskQueue)[0]
		if task.nextRun.After(now) {
			break
		}

		heap.Pop(t.taskQueue) // 从队列中移除

		// 任务已取消
		if !task.active.Load() {
			continue
		}

		// 执行任务
		t.wg.Add(1)
		go t.executeTask(task, now)
	}
}

// 执行任务 - 根据任务类型处理重复任务
func (t *taskScheduler) executeTask(task *scheduledTask, executedAt time.Time) {
	defer t.wg.Done()
	defer RecoverFromError(nil)

	// 执行任务函数
	safeCall(t.executor, task.execute)

	// 单次任务执行后立即清理
	if task.taskType == TaskOnce {
		t.Cancel(task.id) // 使用Cancel确保原子操作
		return
	}

	// 检查任务是否仍然活跃
	if !task.active.Load() {
		return
	}

	t.queueMu.Lock()
	defer t.queueMu.Unlock()

	// 计算下次执行时间
	switch task.taskType {
	case TaskFixedRate:
		task.nextRun = executedAt.Add(task.interval)
	case TaskFixedDelay:
		task.nextRun = time.Now().Add(task.interval)
	}

	// 重新入队
	heap.Push(t.taskQueue, task)

	// 唤醒调度器处理新任务
	select {
	case t.wakeCh <- struct{}{}:
	default:
	}
}

// 添加任务
func (t *taskScheduler) addTask(delay time.Duration, interval time.Duration, taskType TaskType, f func()) int64 {
	if !t.running.Load() {
		log.Warn("scheduler not running, task not scheduled")
		return -1
	}

	taskID := t.seq.Add(1)
	nextRun := time.Now().Add(delay)

	task := &scheduledTask{
		id:       taskID,
		taskType: taskType,
		execute:  f,
		nextRun:  nextRun,
		interval: interval,
	}
	task.active.Store(true)

	t.tasks.Store(taskID, task)
	t.count.Add(1)

	t.queueMu.Lock()
	heap.Push(t.taskQueue, task)
	t.queueMu.Unlock()

	// 唤醒调度器处理新任务
	select {
	case t.wakeCh <- struct{}{}:
	default:
	}

	return taskID
}

// Once 单次任务
func (t *taskScheduler) Once(delay time.Duration, f func()) int64 {
	return t.addTask(delay, 0, TaskOnce, f)
}

// Schedule 指定时间执行
func (t *taskScheduler) Schedule(at time.Time, f func()) int64 {
	delay := time.Until(at)
	if delay < 0 {
		delay = 0
	}
	return t.Once(delay, f)
}

// FixedRate 固定速率任务 (基于起始时间)
func (t *taskScheduler) FixedRate(delay, interval time.Duration, f func()) int64 {
	return t.addTask(delay, interval, TaskFixedRate, f)
}

// FixedDelay 固定延迟任务 (基于结束时间)
func (t *taskScheduler) FixedDelay(delay, interval time.Duration, f func()) int64 {
	return t.addTask(delay, interval, TaskFixedDelay, f)
}

// Forever 重复任务 (固定速率)
func (t *taskScheduler) Forever(interval time.Duration, f func()) int64 {
	return t.FixedRate(interval, interval, f)
}

// ForeverNow 立即执行后重复 (固定速率)
func (t *taskScheduler) ForeverNow(interval time.Duration, f func()) int64 {
	safeCall(t.executor, f)
	return t.FixedRate(interval, interval, f)
}

// Cancel 取消任务 (原子操作优化)
func (t *taskScheduler) Cancel(taskID int64) {
	if task, ok := t.tasks.LoadAndDelete(taskID); ok {
		tsk := task.(*scheduledTask)
		if tsk.active.CompareAndSwap(true, false) {
			t.count.Add(-1)
		}
	}
}

// CancelAll 取消所有任务
func (t *taskScheduler) CancelAll() {
	t.tasks.Range(func(key, value any) bool {
		if tsk, ok := value.(*scheduledTask); ok {
			if tsk.active.CompareAndSwap(true, false) {
				t.count.Add(-1)
			}
			t.tasks.Delete(key)
		}
		return true
	})
}

// Stop 关闭调度器
func (t *taskScheduler) Stop() {
	t.stopOnce.Do(func() {
		t.running.Store(false)
		t.cancel()
		t.CancelAll()

		// 唤醒调度器退出
		select {
		case t.wakeCh <- struct{}{}:
		default:
		}

		// 等待所有任务完成
		t.wg.Wait()
	})
}

// Len 活跃任务数量 (准确计数)
func (t *taskScheduler) Len() int {
	return int(t.count.Load())
}

func (t *taskScheduler) Running() int32 {
	// 返回当前正在执行的任务数量
	// 注意：这不是活跃任务总数，而是当前正在执行的任务数
	return 0 // 示例值，实际实现需要跟踪
}

// safeCall 安全执行函数
func safeCall(executor ITaskExecutor, f func()) {
	if executor != nil {
		executor.Post(func() {
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

// RecoverFromError 恢复panic
func RecoverFromError(cb func(e any)) {
	if e := recover(); e != nil {
		log.Errorf("Recover => %v\n%s\n", e, debug.Stack())
		if cb != nil {
			cb(e)
		}
	}
}
