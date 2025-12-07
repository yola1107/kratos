package work

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type heapMockScheduler struct {
	context.Context
	context.CancelFunc
	iMockExecutor
	Scheduler
}

func (s *heapMockScheduler) Stop() {
	s.CancelFunc()
	s.Scheduler.Stop()
	s.iMockExecutor.Stop()
}

func createHeapScheduler(b *testing.B, timeout time.Duration) (context.Context, context.CancelFunc, *heapMockScheduler) {
	const size = 10000
	// ants
	// loop := NewLoop(WithSize(size))
	// _ = loop.Start()

	// goroutine
	loop := newMockExecutor(size)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	timer := NewHeapScheduler(WithHeapExecutor(loop), WithHeapContext(ctx))

	return ctx, cancel, &heapMockScheduler{
		Context:       ctx,
		CancelFunc:    cancel,
		Scheduler:     timer,
		iMockExecutor: loop,
	}
}

func heapTaskDelay(i int) time.Duration {
	return defaultHeapTickPrecision + time.Duration(i%3)*defaultHeapTickPrecision
}

// --- Benchmark 1: Once tasks (Heap Scheduler)
func BenchmarkHeapOnceTasks(b *testing.B) {
	ctx, cancel, scheduler := createHeapScheduler(b, 3*time.Second)
	defer cancel()
	defer scheduler.Stop()

	var counter int64
	done := make(chan struct{})
	var onceClose sync.Once
	target := int64(b.N)

	for i := 0; i < b.N; i++ {
		delay := heapTaskDelay(i)
		scheduler.Once(delay, func() {
			if atomic.AddInt64(&counter, 1) == target {
				onceClose.Do(func() { close(done) })
			}
		})

		if i%100 == 0 {
			time.Sleep(time.Millisecond)
		}
	}

	select {
	case <-done:
	case <-ctx.Done():
		b.Fatal("timeout waiting for once tasks")
	}
}

// --- Benchmark 2: Forever tasks (Heap Scheduler)
func BenchmarkHeapForeverTasks(b *testing.B) {
	ctx, cancel, scheduler := createHeapScheduler(b, 3*time.Second)
	defer cancel()
	defer scheduler.Stop()

	var counter int64
	done := make(chan struct{})
	var onceClose sync.Once
	target := int64(b.N)

	for i := 0; i < b.N; i++ {
		delay := heapTaskDelay(i)
		scheduler.Forever(delay, func() {
			if atomic.AddInt64(&counter, 1) == target {
				onceClose.Do(func() { close(done) })
			}
		})

		if i%100 == 0 {
			time.Sleep(time.Millisecond)
		}
	}

	select {
	case <-done:
	case <-ctx.Done():
		b.Fatal("timeout waiting for forever tasks")
	}
}

// --- Benchmark 3: Mixed tasks (Heap Scheduler)
func BenchmarkHeapMixedTasks(b *testing.B) {
	ctx, cancel, scheduler := createHeapScheduler(b, 5*time.Second)
	defer cancel()
	defer scheduler.Stop()

	var onceCounter, foreverCounter int64
	done := make(chan struct{})
	var onceClose sync.Once
	n := b.N

	for i := 0; i < n; i++ {
		onceDelay := heapTaskDelay(i)
		foreverDelay := defaultHeapTickPrecision + time.Duration(i%5)*defaultHeapTickPrecision

		scheduler.Once(onceDelay, func() {
			atomic.AddInt64(&onceCounter, 1)
		})

		if i%3 == 0 {
			scheduler.Forever(foreverDelay, func() {
				if atomic.AddInt64(&foreverCounter, 1) >= int64(n*10) {
					onceClose.Do(func() { close(done) })
				}
			})
		}

		if i%100 == 0 {
			time.Sleep(time.Millisecond)
		}
	}

	select {
	case <-done:
		b.Logf("Once: %d, Forever: %d", atomic.LoadInt64(&onceCounter), atomic.LoadInt64(&foreverCounter))
	case <-ctx.Done():
		b.Fatal("timeout waiting for mixed tasks")
	}
}

func BenchmarkHeapSchedulerPrecision(b *testing.B) {
	const totalTasks = 10000 // 堆调度器适合较少任务，这里减少任务数量

	_, cancel, scheduler := createHeapScheduler(b, 15*time.Second) // 增加超时时间
	defer cancel()
	defer scheduler.Stop()

	var (
		mu       sync.Mutex
		delays   = make([]time.Duration, 0, totalTasks)
		executed int64
		wg       sync.WaitGroup
	)

	wg.Add(totalTasks)
	startTime := time.Now()

	for i := 0; i < totalTasks; i++ {
		delay := heapTaskDelay(i)
		expect := startTime.Add(delay)

		scheduler.Once(delay, func() {
			diff := time.Since(expect)
			if diff < 0 {
				diff = 0
			}
			mu.Lock()
			delays = append(delays, diff)
			mu.Unlock()
			atomic.AddInt64(&executed, 1)
			wg.Done()
		})
	}

	wg.Wait()
	b.ReportAllocs()

	if executed == 0 {
		b.Fatal("no task executed")
	}

	mu.Lock()
	defer mu.Unlock()

	minDelay, maxDelay := delays[0], delays[0]
	var totalDelay time.Duration
	for _, d := range delays {
		if d < minDelay {
			minDelay = d
		}
		if d > maxDelay {
			maxDelay = d
		}
		totalDelay += d
	}
	avgDelay := totalDelay / time.Duration(len(delays))

	b.Logf("Executed tasks: %d", executed)
	b.Logf("Min delay error: %v", minDelay)
	b.Logf("Max delay error: %v", maxDelay)
	b.Logf("Avg delay error: %.3f ms", float64(avgDelay.Microseconds())/1000)
}
