package work

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// go test -v -bench=. -benchmem

// mockExecutor 是一个简单的 ITaskExecutor 实现，直接在 goroutine 中执行任务。
type mockExecutor struct{}

func (m *mockExecutor) Post(job func()) {
	go job()
}

// BenchmarkOnceTasks 测试大量单次任务的调度开销
func BenchmarkOnceTasks(b *testing.B) {
	executor := &mockExecutor{}
	scheduler := NewTaskScheduler(executor, context.Background())
	defer scheduler.Shutdown()

	var counter int64
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		scheduler.Once(time.Millisecond, func() {
			atomic.AddInt64(&counter, 1)
		})
	}

	// 等待所有任务执行完
	for {
		if int(atomic.LoadInt64(&counter)) >= b.N {
			break
		}
		time.Sleep(1 * time.Millisecond)
	}
}

// BenchmarkRepeatedTasks 测试高频重复任务的调度精度和负载
func BenchmarkRepeatedTasks(b *testing.B) {
	executor := &mockExecutor{}
	scheduler := NewTaskScheduler(executor, context.Background())
	defer scheduler.Shutdown()

	var counter int64
	var wg sync.WaitGroup
	wg.Add(b.N)

	for i := 0; i < b.N; i++ {
		scheduler.Forever(time.Millisecond*5, func() {
			if atomic.AddInt64(&counter, 1) <= int64(b.N) {
				wg.Done()
			}
		})
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// pass
	case <-time.After(5 * time.Second):
		b.Fatal("timeout waiting for repeated tasks")
	}
}

// TestSchedulerPrecision 精度测试
func TestSchedulerPrecision(t *testing.T) {
	executor := &mockExecutor{}
	scheduler := NewTaskScheduler(executor, context.Background())
	defer scheduler.Shutdown()

	start := time.Now()
	delay := 100 * time.Millisecond
	var firedAt time.Time
	var wg sync.WaitGroup
	wg.Add(1)

	scheduler.Once(delay, func() {
		firedAt = time.Now()
		wg.Done()
	})

	wg.Wait()
	actualDelay := firedAt.Sub(start)
	diff := actualDelay - delay
	if diff > 20*time.Millisecond || diff < -20*time.Millisecond {
		t.Errorf("scheduler precision deviation too high: expected ~%v, got %v (diff: %v)", delay, actualDelay, diff)
	}
}
