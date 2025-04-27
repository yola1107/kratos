package work

import (
	"context"
	"testing"
)

// go test -bench=. -benchmem ./work

// 测试用简单任务（模拟轻量级计算）
func mockJob() {
	_ = 1 + 1
}

func mockCtxJob() func() {
	return func() {
		_ = 1 + 1
	}
}

func BenchmarkAntsLoop_Post(b *testing.B) {
	loop := NewAntsLoop(128) // 使用 128 个协程池大小
	if err := loop.Start(); err != nil {
		b.Fatalf("start failed: %v", err)
	}
	defer loop.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		loop.Post(mockJob)
	}
}

func BenchmarkAntsLoop_PostCtx(b *testing.B) {
	ctx := context.Background()
	loop := NewAntsLoop(128)
	if err := loop.Start(); err != nil {
		b.Fatalf("start failed: %v", err)
	}
	defer loop.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		loop.PostCtx(ctx, mockCtxJob())
	}
}

func BenchmarkAntsLoop_PostAndWait(b *testing.B) {
	loop := NewAntsLoop(128)
	if err := loop.Start(); err != nil {
		b.Fatalf("start failed: %v", err)
	}
	defer loop.Stop()

	job := func() ([]byte, error) {
		return []byte("ok"), nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := loop.PostAndWait(job)
		if err != nil {
			b.Errorf("job failed: %v", err)
		}
	}
}

func BenchmarkAntsLoop_PostAndWaitCtx(b *testing.B) {
	loop := NewAntsLoop(128)
	if err := loop.Start(); err != nil {
		b.Fatalf("start failed: %v", err)
	}
	defer loop.Stop()

	ctx := context.Background()
	job := func() ([]byte, error) {
		return []byte("ok"), nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := loop.PostAndWaitCtx(ctx, job)
		if err != nil {
			b.Errorf("job failed: %v", err)
		}
	}
}
