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

// TaskType 定义任务类型
type TaskType int

const (
	TaskOnce TaskType = iota
	TaskRepeated
)

// ITaskScheduler 调度器接口
type ITaskScheduler interface {
	Len() int
	Once(delay time.Duration, f func()) int64
	Schedule(at time.Time, f func()) int64
	Forever(interval time.Duration, f func()) int64
	ForeverNow(interval time.Duration, f func()) int64
	ForeverTime(firstDelay, interval time.Duration, f func()) int64
	Cancel(taskID int64)
	CancelAll()
	Shutdown()
}

// ITaskExecutor 任务执行器接口
type ITaskExecutor interface {
	Post(job func())
}

// taskScheduler 实现
type taskScheduler struct {
	tasks     sync.Map           // 存储所有任务 [int64]*scheduledTask
	seq       atomic.Int64       // 任务ID计数器
	count     atomic.Int32       // 活跃任务计数器
	executor  ITaskExecutor      // 任务执行器
	ctx       context.Context    // 根上下文
	cancel    context.CancelFunc // 取消函数
	taskQueue *taskPriorityQueue // 任务优先队列
	wg        sync.WaitGroup     // 等待组
	stopOnce  sync.Once          // 确保关闭只执行一次
	queueMu   sync.Mutex         // 队列锁
	cond      *sync.Cond         // 条件变量
	running   atomic.Bool        // 调度器运行状态
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
func NewTaskScheduler(executor ITaskExecutor, parentCtx context.Context) ITaskScheduler {
	ctx, cancel := context.WithCancel(parentCtx)
	s := &taskScheduler{
		executor:  executor,
		ctx:       ctx,
		cancel:    cancel,
		taskQueue: &taskPriorityQueue{},
	}
	s.cond = sync.NewCond(&s.queueMu)
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

// 调度器主循环
func (t *taskScheduler) schedulerLoop() {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("scheduler panic: %v\n%s", r, debug.Stack())
			if t.running.Load() {
				// 尝试重启调度器
				go t.schedulerLoop()
			}
		}
	}()

	for {
		if !t.running.Load() {
			return
		}

		// 等待下一个任务或唤醒信号
		nextWake := t.waitForNextTask()
		if nextWake == nil {
			return // 调度器已关闭
		}

		// 计算等待时间
		waitDuration := time.Until(*nextWake)
		if waitDuration <= 0 {
			t.processDueTasks()
			continue
		}

		select {
		case <-time.After(waitDuration):
			t.processDueTasks()
		case <-t.ctx.Done():
			return
		}
	}
}

// 等待下一个任务
func (t *taskScheduler) waitForNextTask() *time.Time {
	t.queueMu.Lock()
	defer t.queueMu.Unlock()

	for {
		if !t.running.Load() {
			return nil
		}

		if t.taskQueue.Len() > 0 {
			nextTask := (*t.taskQueue)[0]
			return &nextTask.nextRun
		}

		// 队列为空，等待新任务
		t.cond.Wait()
	}
}

// 处理到期任务
func (t *taskScheduler) processDueTasks() {
	t.queueMu.Lock()
	defer t.queueMu.Unlock()

	now := time.Now()
	for t.taskQueue.Len() > 0 {
		task := heap.Pop(t.taskQueue).(*scheduledTask)

		// 任务未到期，放回队列
		if task.nextRun.After(now) {
			heap.Push(t.taskQueue, task)
			return
		}

		// 任务已取消
		if !task.active.Load() {
			// 任务已取消，从全局任务映射中删除
			t.tasks.Delete(task.id)
			continue
		}

		// 执行任务
		t.wg.Add(1)
		go t.executeTask(task, now)
	}
}

// 执行任务 - 修复计数问题
func (t *taskScheduler) executeTask(task *scheduledTask, executedAt time.Time) {
	defer t.wg.Done()
	defer RecoverFromError(nil)

	// 执行任务函数
	safeCall(t.executor, task.execute)

	// 处理单次任务：执行后立即清理
	if task.taskType == TaskOnce {
		task.active.Store(false)
		t.tasks.Delete(task.id)
		t.count.Add(-1)
		return
	}

	// 处理重复任务
	if task.taskType == TaskRepeated && task.active.Load() {
		nextRun := executedAt.Add(task.interval)

		// 更新任务时间并重新加入队列
		t.queueMu.Lock()
		task.nextRun = nextRun
		heap.Push(t.taskQueue, task)
		t.cond.Signal() // 唤醒调度器
		t.queueMu.Unlock()
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
	t.count.Add(1) // 增加活跃任务计数

	t.queueMu.Lock()
	heap.Push(t.taskQueue, task)
	t.cond.Signal() // 唤醒调度器
	t.queueMu.Unlock()

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

// Forever 重复任务
func (t *taskScheduler) Forever(interval time.Duration, f func()) int64 {
	return t.addTask(interval, interval, TaskRepeated, f)
}

// ForeverNow 立即执行后重复
func (t *taskScheduler) ForeverNow(interval time.Duration, f func()) int64 {
	safeCall(t.executor, f)
	return t.Forever(interval, f)
}

// ForeverTime 首次延迟不同的重复任务
func (t *taskScheduler) ForeverTime(firstDelay, interval time.Duration, f func()) int64 {
	return t.addTask(firstDelay, interval, TaskRepeated, f)
}

// Cancel 取消任务 - 修复计数问题
func (t *taskScheduler) Cancel(taskID int64) {
	if task, ok := t.tasks.Load(taskID); ok {
		if tsk, ok := task.(*scheduledTask); ok {
			tsk.active.Store(false)
			t.tasks.Delete(taskID)
			t.count.Add(-1) // 减少活跃任务计数
		}
	}
}

// CancelAll 取消所有任务 - 修复计数问题
func (t *taskScheduler) CancelAll() {
	t.tasks.Range(func(key, value any) bool {
		if tsk, ok := value.(*scheduledTask); ok {
			tsk.active.Store(false)
			t.tasks.Delete(key)
			t.count.Add(-1) // 减少活跃任务计数
		}
		return true
	})
}

// Shutdown 关闭调度器
func (t *taskScheduler) Shutdown() {
	t.stopOnce.Do(func() {
		t.running.Store(false)
		t.cancel()
		t.CancelAll() // 取消所有任务

		// 唤醒可能等待的调度器
		t.queueMu.Lock()
		t.cond.Broadcast()
		t.queueMu.Unlock()

		// 等待所有任务完成
		t.wg.Wait()
	})
}

// Len 活跃任务数量 - 现在计数器是准确的
func (t *taskScheduler) Len() int {
	return int(t.count.Load())
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
