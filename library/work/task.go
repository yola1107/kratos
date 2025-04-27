package work

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/panjf2000/ants/v2"

	"github.com/yola1107/kratos/v2/log"
)

// 定义结果类型
type asyncResult struct {
	data []byte
	err  error
}

// ITaskLoop 协程池管理接口
type ITaskLoop interface {
	Start() error
	Stop()
	Post(job func())
	PostCtx(ctx context.Context, job func())
	PostAndWait(job func() ([]byte, error)) ([]byte, error)
	PostAndWaitCtx(ctx context.Context, job func() ([]byte, error)) ([]byte, error)
}

type Option func(*antsLoop)

// WithFallback 自定义任务提交失败处理策略
func WithFallback(fallback func(ctx context.Context, fn func())) Option {
	return func(l *antsLoop) {
		l.fallback = fallback
	}
}

type antsLoop struct {
	mu       sync.RWMutex
	pool     *ants.Pool
	size     int
	fallback func(context.Context, func())
}

// NewAntsLoop 创建协程池实例
func NewAntsLoop(size int, opts ...Option) ITaskLoop {
	l := &antsLoop{
		size: size,
		fallback: func(ctx context.Context, fn func()) {
			go safeRun(ctx, fn)
		},
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

func (l *antsLoop) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.pool != nil {
		log.Warnf("antsLoop already started.")
		return nil
	}

	pool, err := ants.NewPool(l.size)
	if err != nil {
		return fmt.Errorf("pool init failed: %w", err)
	}

	l.pool = pool
	log.Infof("antsLoop start... [size:%d]", l.size)
	return nil
}

func (l *antsLoop) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.pool != nil {
		p := l.pool
		l.pool = nil
		p.Release()
		log.Infof("antsLoop stopping [running:%d]", p.Running())
	}
}

func (l *antsLoop) Post(job func()) {
	l.PostCtx(context.Background(), job)
}

func (l *antsLoop) PostCtx(ctx context.Context, job func()) {
	if ctx.Err() == nil {
		l.submit(ctx, job)
	}
}

func (l *antsLoop) PostAndWait(job func() ([]byte, error)) ([]byte, error) {
	return l.PostAndWaitCtx(context.Background(), job)
}

func (l *antsLoop) PostAndWaitCtx(ctx context.Context, job func() ([]byte, error)) ([]byte, error) {
	ch := make(chan *asyncResult, 1)

	l.submit(ctx, func() {
		defer RecoverFromError(func(e any) {
			// 通过select确保panic信息能发送出去, 防止调用方一直阻塞等待接收job的返回结果
			select {
			case ch <- &asyncResult{nil, fmt.Errorf("panic: %v", e)}:
			default:
			}
		})
		data, err := job()
		select {
		case ch <- &asyncResult{data, err}:
		case <-ctx.Done():
		}
	})

	select {
	case res := <-ch:
		return res.data, res.err
	case <-ctx.Done():
		select {
		case res := <-ch:
			return res.data, res.err
		default:
			// 确保job被取消的信息能发送出去, 防止调用方一直阻塞等待接收job的返回结果
			return nil, fmt.Errorf("canceled: %w", ctx.Err())
		}
	}
}

func (l *antsLoop) submit(ctx context.Context, fn func()) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.pool == nil || l.pool.IsClosed() {
		l.triggerFallback(ctx, fn, "loop not started or loop is closed.")
		return
	}

	if err := l.pool.Submit(func() { safeRun(ctx, fn) }); err != nil {
		l.triggerFallback(ctx, fn, err.Error())
	}
}

func (l *antsLoop) triggerFallback(ctx context.Context, fn func(), reason string) {
	log.Warnf("ansloop fallback. reason=%s", reason)
	l.fallback(ctx, fn)
}

func safeRun(ctx context.Context, fn func()) {
	defer RecoverFromError(nil)
	if ctx.Err() == nil {
		fn()
	}
}

func RecoverFromError(cb func(e any)) {
	if e := recover(); e != nil {
		log.Errorf("Recover => %v\n%s\n", e, debug.Stack())
		if cb != nil {
			cb(e)
		}
	}
}
