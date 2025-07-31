package xgo

import (
	"context"
	"time"
)

// ReTry 重试逻辑，最多重试n次
func ReTry(n int, f func() error) error {
	if f == nil {
		return nil
	}
	var err error
	for i := 0; i < n; i++ {
		err = f()
		if err == nil {
			return nil
		}
	}
	return err
}

// ReTryWithDelay 尝试执行 f 最多 n 次，每次失败等待 delay，支持 context 取消
func ReTryWithDelay(ctx context.Context, n int, delay time.Duration, f func() error) error {
	if f == nil {
		return nil
	}
	var err error
	for i := 0; i < n; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = f()
		if err == nil {
			return nil
		}
		// 如果不是最后一次才等待
		if i < n-1 && delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return err
}
