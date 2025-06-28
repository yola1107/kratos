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

func TestAntsLoop(t *testing.T) {
	l := NewAntsLoop(2)
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

	executor := &mockExecutor{}
	scheduler := NewTaskScheduler(executor, ctx)
	defer scheduler.Shutdown()

	t.Run("Once task executes", func(t *testing.T) {
		done := make(chan struct{})
		scheduler.Once(10*time.Millisecond, func() {
			close(done)
		})
		waitForChannel(t, done, 100*time.Millisecond, "Once task did not execute")
	})

	t.Run("Forever task repeats", func(t *testing.T) {
		start := time.Now()
		t.Logf("Test started at: %s", start.Format(time.RFC3339Nano))

		var count atomic.Int32
		done := make(chan struct{})

		id := scheduler.Forever(20*time.Millisecond, func() {
			current := count.Add(1)
			t.Logf("Task executed | Count: %d | Elapsed: %v", current, time.Since(start))

			if current >= 3 {
				close(done)
			}
		})

		waitForChannel(t, done, 200*time.Millisecond, "Forever task timed out")
		require.GreaterOrEqual(t, count.Load(), int32(3), "Expected at least 3 executions")

		// Test cancellation
		t.Logf("Cancelling task at: %v", time.Since(start))
		scheduler.Cancel(id)

		prev := count.Load()
		time.Sleep(100 * time.Millisecond) // Wait to ensure no further executions
		require.Equal(t, prev, count.Load(), "Task continued after cancellation")
	})

	t.Run("ForeverNow executes immediately", func(t *testing.T) {
		start := time.Now()
		first := make(chan struct{})

		id := scheduler.ForeverNow(50*time.Millisecond, func() {
			if count := time.Since(start); count < 20*time.Millisecond {
				close(first)
			}
		})

		waitForChannel(t, first, 20*time.Millisecond, "ForeverNow did not execute immediately")
		scheduler.Cancel(id)
	})
}

func TestTaskScheduler_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := &mockExecutor{}
	scheduler := NewTaskScheduler(executor, ctx)
	defer scheduler.Shutdown()

	t.Run("Cancel single task", func(t *testing.T) {
		var executed atomic.Bool
		id := scheduler.Once(20*time.Millisecond, func() {
			executed.Store(true)
		})

		scheduler.Cancel(id)
		time.Sleep(50 * time.Millisecond)
		require.False(t, executed.Load(), "Cancelled task was executed")
	})

	t.Run("CancelAll stops all tasks", func(t *testing.T) {
		const taskCount = 5
		var executed atomic.Int32

		for i := 0; i < taskCount; i++ {
			scheduler.Once(30*time.Millisecond, func() {
				executed.Add(1)
			})
		}

		scheduler.CancelAll()
		time.Sleep(50 * time.Millisecond)
		require.Zero(t, executed.Load(), "Expected 0 executions after CancelAll")
	})
}

func TestTaskScheduler_Shutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := &mockExecutor{}
	scheduler := NewTaskScheduler(executor, ctx)

	// Add tasks that should not execute
	scheduler.Once(100*time.Millisecond, func() { t.Error("Once task executed after shutdown") })
	scheduler.Forever(50*time.Millisecond, func() { t.Error("Forever task executed after shutdown") })

	// Shutdown immediately
	scheduler.Shutdown()

	// Try to add new task
	id := scheduler.Once(10*time.Millisecond, func() {
		t.Error("New task executed after Shutdown")
	})
	require.Equal(t, int64(-1), id, "Expected -1 when scheduling after shutdown")

	// Ensure no tasks executed
	time.Sleep(200 * time.Millisecond)
}

func TestTaskScheduler_PanicRecovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := &mockExecutor{}
	scheduler := NewTaskScheduler(executor, ctx)
	defer scheduler.Shutdown()

	t.Run("Recover from panic in Once task", func(t *testing.T) {
		done := make(chan struct{})
		scheduler.Once(10*time.Millisecond, func() {
			defer close(done)
			panic("test panic")
		})
		waitForChannel(t, done, 100*time.Millisecond, "Task did not execute")
	})

	t.Run("Recover from panic in Forever task", func(t *testing.T) {
		done := make(chan struct{})
		var count atomic.Int32

		id := scheduler.Forever(20*time.Millisecond, func() {
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

		waitForChannel(t, done, 200*time.Millisecond, "Forever task timed out")
		scheduler.Cancel(id)
		require.GreaterOrEqual(t, count.Load(), int32(2), "Task should have executed multiple times")
	})
}

func TestTaskScheduler_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	executor := &mockExecutor{}
	scheduler := NewTaskScheduler(executor, ctx)
	defer scheduler.Shutdown()

	t.Run("Context cancel stops scheduler", func(t *testing.T) {
		var executed atomic.Bool
		scheduler.Once(100*time.Millisecond, func() {
			executed.Store(true)
		})

		cancel() // Cancel parent context
		time.Sleep(150 * time.Millisecond)
		require.False(t, executed.Load(), "Task should not execute after context cancel")
	})
}

// Helper function to wait for a channel with timeout
func waitForChannel(t *testing.T, ch <-chan struct{}, timeout time.Duration, failMsg string) {
	select {
	case <-ch:
		// Success
	case <-time.After(timeout):
		t.Fatal(failMsg)
	}
}
