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

func createScheduler(b *testing.B, timeout time.Duration) (context.Context, context.CancelFunc, ITaskScheduler) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	scheduler := NewTaskScheduler(&mockExecutor{}, ctx)
	return ctx, cancel, scheduler
}

func BenchmarkOnceTasks(b *testing.B) {
	ctx, cancel, scheduler := createScheduler(b, 3*time.Second)
	defer cancel()
	defer scheduler.Stop()

	var counter int64
	done := make(chan struct{})
	target := int64(b.N)

	for i := 0; i < b.N; i++ {
		delay := defaultTickPrecision + time.Duration(i%3)*defaultTickPrecision // 100ms ~ 300ms
		scheduler.Once(delay, func() {
			if atomic.AddInt64(&counter, 1) == target {
				close(done)
			}
		})
	}

	select {
	case <-done:
	case <-ctx.Done():
		b.Fatal("timeout waiting for once tasks")
	}
}

func BenchmarkForeverTasks(b *testing.B) {
	ctx, cancel, scheduler := createScheduler(b, 3*time.Second)
	defer cancel()
	defer scheduler.Stop()

	var counter int64
	done := make(chan struct{})
	target := int64(b.N)

	for i := 0; i < b.N; i++ {
		delay := defaultTickPrecision + time.Duration(i%3)*defaultTickPrecision // 100ms ~ 300ms
		scheduler.Forever(delay, func() {
			if atomic.AddInt64(&counter, 1) == target {
				close(done)
			}
		})
	}

	select {
	case <-done:
	case <-ctx.Done():
		b.Fatal("timeout waiting for forever tasks")
	}
}

func BenchmarkMixedTasks(b *testing.B) {
	ctx, cancel, scheduler := createScheduler(b, 5*time.Second)
	defer cancel()
	defer scheduler.Stop()

	var onceCounter, foreverCounter int64
	done := make(chan struct{})
	var once sync.Once

	n := b.N // 避免闭包中并发访问 b.N

	b.ResetTimer()

	for i := 0; i < n; i++ {
		onceDelay := defaultTickPrecision + time.Duration(i%3)*defaultTickPrecision // 100ms ~ 300ms
		foreverDelay := defaultTickPrecision + time.Duration(i%5)*defaultTickPrecision

		scheduler.Once(onceDelay, func() {
			atomic.AddInt64(&onceCounter, 1)
		})
		if i%3 == 0 {
			scheduler.Forever(foreverDelay, func() {
				val := atomic.AddInt64(&foreverCounter, 1)
				if val >= int64(n*10) {
					once.Do(func() {
						close(done)
					})
				}
			})
		}
	}

	select {
	case <-done:
		b.Logf("Once: %d, Forever: %d", atomic.LoadInt64(&onceCounter), atomic.LoadInt64(&foreverCounter))
	case <-ctx.Done():
		b.Fatal("timeout waiting for mixed tasks")
	}
}

func BenchmarkSchedulerPrecision(b *testing.B) {
	ctx, cancel, scheduler := createScheduler(b, 2*time.Second)
	defer cancel()
	defer scheduler.Stop()

	var delayErrors []time.Duration
	var mu sync.Mutex
	var executed int64
	target := b.N

	for i := 0; i < b.N; i++ {
		delay := defaultTickPrecision + time.Duration(i%3)*defaultTickPrecision // 100ms ~ 300ms
		expect := time.Now().Add(delay)

		scheduler.Once(delay, func() {
			actual := time.Since(expect)
			mu.Lock()
			delayErrors = append(delayErrors, actual)
			mu.Unlock()

			if atomic.AddInt64(&executed, 1) == int64(target) {
				cancel()
			}
		})
	}

	<-ctx.Done()

	if executed == 0 {
		b.Fatal("no task executed")
	}

	var min, max, sum time.Duration
	min = delayErrors[0]
	max = delayErrors[0]
	for _, d := range delayErrors {
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
		sum += d
	}
	avg := sum / time.Duration(len(delayErrors))

	b.Logf("Executed %d tasks", executed)
	b.Logf("Min delay error: %v", min)
	b.Logf("Max delay error: %v", max)
	b.Logf("Avg delay error: %v", avg)
}
