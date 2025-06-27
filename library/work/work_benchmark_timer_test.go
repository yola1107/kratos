package work

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockExecutor 直接在 goroutine 中执行任务
type mockExecutor struct{}

func (m *mockExecutor) Post(job func()) {
	go job()
}

func createScheduler(tb testing.TB, timeout time.Duration) (context.Context, context.CancelFunc, ITaskScheduler) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	executor := &mockExecutor{}
	scheduler := NewTaskScheduler(executor, ctx)
	return ctx, cancel, scheduler
}

func BenchmarkOnceTasks(b *testing.B) {
	_, cancel, scheduler := createScheduler(b, 5*time.Second)
	defer cancel()
	defer scheduler.Shutdown()

	var counter int64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.Once(10*time.Millisecond, func() {
			atomic.AddInt64(&counter, 1)
		})
	}

	for start := time.Now(); ; {
		if atomic.LoadInt64(&counter) == int64(b.N) {
			break
		}
		if time.Since(start) > 5*time.Second {
			b.Fatalf("timeout: only %d/%d tasks completed", counter, b.N)
		}
		time.Sleep(1 * time.Millisecond)
	}
}

func BenchmarkForeverTasks(b *testing.B) {
	ctx, cancel, scheduler := createScheduler(b, 3*time.Second)
	defer cancel()
	defer scheduler.Shutdown()

	var counter int64
	done := make(chan struct{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.Forever(20*time.Millisecond, func() {
			if atomic.AddInt64(&counter, 1) == int64(b.N*5) {
				close(done)
			}
		})
	}

	select {
	case <-done:
		b.Logf("Completed %d executions", counter)
	case <-ctx.Done():
		b.Fatal("timeout waiting for tasks")
	}
}

func BenchmarkMixedTasks(b *testing.B) {
	ctx, cancel, scheduler := createScheduler(b, 5*time.Second)
	defer cancel()
	defer scheduler.Shutdown()

	var onceCounter, foreverCounter int64
	done := make(chan struct{})
	var once sync.Once

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.Once(time.Duration(i%10+1)*time.Millisecond, func() {
			atomic.AddInt64(&onceCounter, 1)
		})
		if i%3 == 0 {
			scheduler.Forever(time.Duration(i%20+10)*time.Millisecond, func() {
				val := atomic.AddInt64(&foreverCounter, 1)
				if val >= int64(b.N*10) {
					once.Do(func() {
						close(done)
					})
				}
			})
		}
	}

	select {
	case <-done:
		b.Logf("Once: %d, Forever: %d", onceCounter, foreverCounter)
	case <-ctx.Done():
		b.Fatal("timeout waiting for tasks")
	}
}

func BenchmarkSchedulerPrecision(b *testing.B) {
	_, cancel, scheduler := createScheduler(b, 10*time.Second)
	defer cancel()
	defer scheduler.Shutdown()

	var (
		mu     sync.Mutex
		errors []time.Duration // 记录所有误差
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduledAt := time.Now().Add(50 * time.Millisecond) // 计划触发时间
		done := make(chan struct{})

		scheduler.Once(50*time.Millisecond, func() {
			actual := time.Now()
			diff := actual.Sub(scheduledAt)

			mu.Lock()
			errors = append(errors, diff)
			mu.Unlock()

			close(done)
		})

		// 等待任务执行完成，避免重叠影响测量
		select {
		case <-done:
		case <-time.After(200 * time.Millisecond):
			b.Fatalf("task timeout")
		}
	}
	b.StopTimer()

	// 统计误差
	var (
		total time.Duration
		max   time.Duration
		min   time.Duration = time.Hour
	)

	for _, d := range errors {
		absd := d
		if d < 0 {
			absd = -d
		}
		if absd > max {
			max = absd
		}
		if absd < min {
			min = absd
		}
		total += absd
	}
	avg := time.Duration(int64(total) / int64(len(errors)))

	b.Logf("Executed %d tasks", len(errors))
	b.Logf("Min delay error: %v", min)
	b.Logf("Max delay error: %v", max)
	b.Logf("Avg delay error: %v", avg)
}
