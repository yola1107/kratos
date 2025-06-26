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

// BenchmarkOnceTasks 测试大量单次任务的调度开销
func BenchmarkOnceTasks(b *testing.B) {
	executor := &mockExecutor{}
	scheduler := NewTaskScheduler(executor, context.Background())
	defer scheduler.CancelAll()

	var counter int64
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		scheduler.Once(time.Millisecond, func() {
			atomic.AddInt64(&counter, 1)
		})
	}

	// 等待所有任务执行完，带超时防止死循环
	timeout := time.After(5 * time.Second)
	for {
		if atomic.LoadInt64(&counter) >= int64(b.N) {
			break
		}
		select {
		case <-timeout:
			b.Fatal("timeout waiting for Once tasks to finish")
		default:
			time.Sleep(1 * time.Millisecond)
		}
	}
}

// BenchmarkForeverTasks 测试高频重复任务的调度精度和负载
func BenchmarkForeverTasks(b *testing.B) {
	executor := &mockExecutor{}
	scheduler := NewTaskScheduler(executor, context.Background())
	defer scheduler.CancelAll()

	var counter int64
	var wg sync.WaitGroup
	wg.Add(b.N)

	for i := 0; i < b.N; i++ {
		scheduler.Forever(time.Millisecond*5, func() {
			val := atomic.AddInt64(&counter, 1)
			if val <= int64(b.N) {
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
		// 测试完成
	case <-time.After(10 * time.Second):
		b.Fatal("timeout waiting for forever tasks")
	}
}

// TestSchedulerPrecision 多次触发测试时间精度并计算偏差
func TestSchedulerPrecision(t *testing.T) {
	executor := &mockExecutor{}
	scheduler := NewTaskScheduler(executor, context.Background())
	defer scheduler.CancelAll()

	const tries = 10
	delay := 100 * time.Millisecond
	var diffs []time.Duration
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(tries)

	for i := 0; i < tries; i++ {
		start := time.Now()
		scheduler.Once(delay, func() {
			actualDelay := time.Since(start)
			diff := actualDelay - delay
			mu.Lock()
			diffs = append(diffs, diff)
			mu.Unlock()
			wg.Done()
		})
		time.Sleep(20 * time.Millisecond) // 适当错开触发时间，防止集中调度
	}

	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for scheduled tasks")
	}

	// 计算最大偏差和平均偏差
	var maxDiff time.Duration
	var totalDiff time.Duration
	for _, d := range diffs {
		if d < 0 {
			d = -d
		}
		if d > maxDiff {
			maxDiff = d
		}
		totalDiff += d
	}
	avgDiff := totalDiff / time.Duration(len(diffs))

	if maxDiff > 20*time.Millisecond {
		t.Errorf("scheduler max precision deviation too high: max %v, avg %v", maxDiff, avgDiff)
	} else {
		t.Logf("scheduler precision max deviation: %v, avg deviation: %v", maxDiff, avgDiff)
	}
}

// TestSchedulerPrecisionStats 统计大量定时任务精度误差
func TestSchedulerPrecisionStats(t *testing.T) {
	executor := &mockExecutor{}
	scheduler := NewTaskScheduler(executor, context.Background())
	defer scheduler.CancelAll()

	const taskCount = 5000
	const delay = 50 * time.Millisecond

	type Result struct {
		delayDiff time.Duration
	}

	results := make([]Result, taskCount)
	var wg sync.WaitGroup
	wg.Add(taskCount)

	start := time.Now()

	for i := 0; i < taskCount; i++ {
		scheduler.Once(delay, func(i int) func() {
			return func() {
				actual := time.Now()
				expected := start.Add(delay)
				diff := actual.Sub(expected)
				results[i].delayDiff = diff
				wg.Done()
			}
		}(i))
	}

	wg.Wait()

	var totalDiff time.Duration
	var maxDiff time.Duration
	var minDiff time.Duration = time.Hour

	for _, r := range results {
		d := r.delayDiff
		if d < 0 {
			d = -d
		}
		totalDiff += d
		if d > maxDiff {
			maxDiff = d
		}
		if d < minDiff {
			minDiff = d
		}
	}

	avgDiff := totalDiff / taskCount

	t.Logf("定时任务精度统计 (误差绝对值)：任务数=%d, 平均误差=%v, 最大误差=%v, 最小误差=%v",
		taskCount, avgDiff, maxDiff, minDiff)
}
