package work

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yola1107/kratos/v2/log"
)

func init() {
	log.SetLogger(log.NewStdLogger(os.Stdout))
}

// mockExecutor 直接在 goroutine 池中执行任务
type mockExecutor struct {
	pool chan struct{}
}

func newMockExecutor(ops ...int) *mockExecutor {
	size := 100
	if len(ops) > 0 && ops[0] > 0 {
		size = ops[0]
	}
	return &mockExecutor{pool: make(chan struct{}, size)}

}

func (m *mockExecutor) Post(job func()) {
	m.pool <- struct{}{}
	go func() {
		defer func() { <-m.pool }()
		job()
	}()
}

func (m *mockExecutor) Stop() {}

func TestAntsLoop(t *testing.T) {
	l := NewLoop(WithSize(2))
	err := l.Start()
	require.NoError(t, err)
	defer l.Stop()

	t.Run("start more times", func(t *testing.T) {
		err = l.Start()
		require.NoError(t, err)
	})

	t.Run("Post simple task", func(t *testing.T) {
		done := make(chan struct{})
		l.Post(func() {
			close(done)
		})
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("task not finished")
		}
	})

	t.Run("PostAndWait returns expected value", func(t *testing.T) {
		val, err := l.PostAndWait(func() ([]byte, error) {
			return []byte("hello"), nil
		})
		require.NoError(t, err)
		require.Equal(t, []byte("hello"), val)
	})

	t.Run("Post panic inside job is recovered", func(t *testing.T) {
		done := make(chan struct{})
		l.Post(func() {
			defer func() {
				t.Log("panic recovered")
				close(done)
			}()
			panic("oops")
		})
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("panic job did not finish")
		}
	})

	t.Run("PostAndWait panic inside job is recovered", func(t *testing.T) {
		done := make(chan struct{})
		d, err := l.PostAndWait(func() ([]byte, error) {
			defer func() {
				t.Log("panic recovered")
				close(done)
			}()
			panic("oops2")
			return []byte("hello"), nil
		})
		t.Logf("panic recovered d=%+v ,err=%+v", d, err)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("panic job did not finish")
		}
	})

	t.Run("fallback when stopped", func(t *testing.T) {
		l.Stop()
		// Stop 异步释放，等一会
		time.Sleep(100 * time.Millisecond)

		executed := make(chan struct{})
		l.PostCtx(context.Background(), func() {
			close(executed)
		})

		select {
		case <-executed:
		case <-time.After(time.Second):
			t.Fatal("fallback job did not run")
		}
		// 重新启动方便后面测试
		err := l.Start()
		require.NoError(t, err)
	})

	t.Run("context cancel works in PostAndWaitCtx", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := l.PostAndWaitCtx(ctx, func() ([]byte, error) {
			time.Sleep(200 * time.Millisecond)
			return []byte("slow"), nil
		})
		require.Error(t, err)
		require.True(t, errors.Is(err, context.DeadlineExceeded))
	})
}

/*
	定时器timer
*/

func TestTaskScheduler_BasicOperations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := newMockExecutor()
	scheduler := NewWheelScheduler(WithExecutor(executor), WithContext(ctx))
	defer scheduler.Stop()

	t.Run("Once task executes", func(t *testing.T) {
		done := make(chan struct{})
		scheduler.Once(defaultTickPrecision, func() {
			close(done)
		})
		waitForChannel(t, done, defaultTickPrecision*2, "Once task did not execute")
	})

	t.Run("Forever task repeats", func(t *testing.T) {
		start := time.Now()
		t.Logf("Test started at: %s", start.Format(time.RFC3339Nano))

		var count atomic.Int32
		done := make(chan struct{})

		id := scheduler.Forever(defaultTickPrecision, func() {
			current := count.Add(1)
			t.Logf("Task executed | Count: %d | Elapsed: %v", current, time.Since(start))

			if current >= 3 {
				close(done)
			}
		})

		waitForChannel(t, done, defaultTickPrecision*5, "Forever task timed out")
		require.GreaterOrEqual(t, count.Load(), int32(3), "Expected at least 3 executions")

		// Test cancellation
		t.Logf("Cancelling task at: %v", time.Since(start))
		scheduler.Cancel(id)

		prev := count.Load()
		time.Sleep(defaultTickPrecision * 2)
		require.Equal(t, prev, count.Load(), "Task continued after cancellation")
	})

	t.Run("ForeverNow executes immediately", func(t *testing.T) {
		start := time.Now()
		first := make(chan struct{})

		id := scheduler.ForeverNow(defaultTickPrecision, func() {
			if time.Since(start) < defaultTickPrecision {
				close(first)
			}
		})

		waitForChannel(t, first, defaultTickPrecision*2, "ForeverNow did not execute immediately")
		scheduler.Cancel(id)
	})
}

func TestTaskScheduler_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := newMockExecutor()
	scheduler := NewWheelScheduler(WithExecutor(executor), WithContext(ctx))
	defer scheduler.Stop()

	t.Run("Cancel single task", func(t *testing.T) {
		var executed atomic.Bool
		id := scheduler.Once(defaultTickPrecision, func() {
			executed.Store(true)
		})
		scheduler.Cancel(id)
		time.Sleep(defaultTickPrecision * 2)
		require.False(t, executed.Load(), "Cancelled task was executed")
	})

	t.Run("CancelAll stops all tasks", func(t *testing.T) {
		const taskCount = 5
		var executed atomic.Int32
		for i := 0; i < taskCount; i++ {
			scheduler.Once(defaultTickPrecision, func() {
				executed.Add(1)
			})
		}
		scheduler.CancelAll()
		time.Sleep(defaultTickPrecision * 2)
		require.Zero(t, executed.Load(), "Expected 0 executions after CancelAll")
	})
}

func TestTaskScheduler_Stop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := newMockExecutor()
	scheduler := NewWheelScheduler(WithExecutor(executor), WithContext(ctx))

	scheduler.Once(defaultTickPrecision, func() { t.Error("Once task executed after shutdown") })
	scheduler.Forever(defaultTickPrecision, func() { t.Error("Forever task executed after shutdown") })

	scheduler.Stop()

	id := scheduler.Once(defaultTickPrecision, func() {
		t.Error("New task executed after Stop")
	})
	require.Equal(t, int64(-1), id, "Expected -1 when scheduling after shutdown")

	time.Sleep(defaultTickPrecision * 3)
}

func TestTaskScheduler_PanicRecovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := newMockExecutor()
	scheduler := NewWheelScheduler(WithExecutor(executor), WithContext(ctx))
	defer scheduler.Stop()

	t.Run("Recover from panic in Once task", func(t *testing.T) {
		done := make(chan struct{})
		scheduler.Once(defaultTickPrecision, func() {
			defer close(done)
			panic("test panic")
		})
		waitForChannel(t, done, defaultTickPrecision*2, "Task did not execute")
	})

	t.Run("Recover from panic in Forever task", func(t *testing.T) {
		done := make(chan struct{})
		var count atomic.Int32
		id := scheduler.Forever(defaultTickPrecision, func() {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("Recovered from panic: %v", r)
				}
				if count.Load() >= 2 {
					close(done)
				}
			}()
			count.Add(1)
			panic("periodic panic")
		})
		waitForChannel(t, done, defaultTickPrecision*5, "Forever task timed out")
		scheduler.Cancel(id)
		require.GreaterOrEqual(t, count.Load(), int32(2), "Task should have executed multiple times")
	})
}

func TestTaskScheduler_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	executor := newMockExecutor()
	scheduler := NewWheelScheduler(WithExecutor(executor), WithContext(ctx))
	defer scheduler.Stop()

	t.Run("Context cancel stops wheelScheduler", func(t *testing.T) {
		var executed atomic.Bool
		scheduler.Once(defaultTickPrecision, func() {
			executed.Store(true)
		})

		cancel()
		time.Sleep(defaultTickPrecision * 3)
		require.False(t, executed.Load(), "Task should not execute after context cancel")
	})
}

func waitForChannel(t *testing.T, ch <-chan struct{}, timeout time.Duration, failMsg string) {
	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatal(failMsg)
	}
}

func TestTaskScheduler_TaskIDSequence_WithCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := newMockExecutor()
	scheduler := NewWheelScheduler(WithExecutor(executor), WithContext(ctx))
	defer scheduler.Stop()

	const count = 5
	cancelIdx := 2      // 取消的任务索引
	cancelEarlyIdx := 1 // 注册过程中取消的任务索引

	type task struct {
		id        int64
		triggered atomic.Bool
		done      chan struct{}
	}

	tasks := make([]*task, 0, count+2)

	for i := 0; i < count; i++ {
		tk := &task{done: make(chan struct{})}
		tk.id = scheduler.Once(defaultTickPrecision*2+time.Duration(i)*defaultTickPrecision, func() {
			defer close(tk.done)
			tk.triggered.Store(true)
			t.Logf("任务 #%d 执行，taskID=%d", i, tk.id)
		})
		tasks = append(tasks, tk)
		t.Logf("注册任务 #%d, taskID=%d", i, tk.id)

		if i == 2 {
			t.Logf("在注册过程中取消任务 #%d, taskID=%d", cancelEarlyIdx, tasks[cancelEarlyIdx].id)
			scheduler.Cancel(tasks[cancelEarlyIdx].id)
		}
	}

	t.Logf("取消任务 #%d, taskID=%d", cancelIdx, tasks[cancelIdx].id)
	scheduler.Cancel(tasks[cancelIdx].id)

	for i := 1; i < len(tasks); i++ {
		require.Greater(t, tasks[i].id, tasks[i-1].id, "任务ID应该递增")
	}

	t11 := &task{done: make(chan struct{})}
	t11.id = scheduler.Once(defaultTickPrecision*5, func() {
		defer close(t11.done)
		t11.triggered.Store(true)
		t.Logf("新任务执行，taskID=%d", t11.id)
	})
	tasks = append(tasks, t11)
	t.Logf("注册新任务, taskID=%d", t11.id)

	timeout := time.After(defaultTickPrecision * 30)
	for i, ta := range tasks {
		if i == cancelIdx || i == cancelEarlyIdx {
			continue
		}
		select {
		case <-ta.done:
		case <-timeout:
			t.Fatalf("任务 #%d 未在超时时间内完成", i)
		}
	}

	cancelledTasks := map[int]bool{cancelIdx: true, cancelEarlyIdx: true}
	for i, ta := range tasks {
		triggered := ta.triggered.Load()
		status := "未触发"
		if triggered {
			status = "已触发"
		}
		t.Logf("任务 #%d taskID=%d [%s]", i, ta.id, status)

		if cancelledTasks[i] {
			require.False(t, triggered, "任务 #%d 已取消，不应触发", i)
		} else {
			require.True(t, triggered, "任务 #%d 未触发", i)
		}
	}

	for i := 0; i < count; i++ {
		require.Less(t, tasks[i].id, t11.id, "新任务的ID应大于所有现有任务ID")
	}

	require.Equal(t, 0, scheduler.Len(), "所有任务已完成，调度器应为空")
}

func TestTaskScheduler_TaskIDSequence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := newMockExecutor()
	scheduler := NewWheelScheduler(WithExecutor(executor), WithContext(ctx))
	defer scheduler.Stop()

	prevID := int64(0)
	for i := 1; i <= 5; i++ {
		i := i // fix closure capture
		id := scheduler.Once(defaultTickPrecision/2, func() {
			t.Logf("exec %d", i)
		})
		t.Logf("注册任务 #%d, taskID=%d", i, id)
		require.Greater(t, id, prevID)
		prevID = id
	}

	time.Sleep(defaultTickPrecision * 2)

	format := "2006-01-02 15:04:05.000"
	t.Logf("start at: %v", time.Now().Format(format))
	start := time.Now()
	done := make(chan struct{})
	scheduler.Once(0, func() {
		t.Logf("fast exec %v %v", time.Since(start), time.Now().Format(format))
		close(done)
	})
	t.Logf("end at: %v", time.Now().Format(format))

	select {
	case <-done:
	case <-time.After(time.Millisecond * 10):
		t.Errorf("fast exec not triggered in time")
	}

	time.Sleep(defaultTickPrecision * 3)
}
