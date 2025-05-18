package work

import (
	"context"
	"errors"
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

// IAntsLoop 协程池管理接口
type IAntsLoop interface {
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
func NewAntsLoop(size int, opts ...Option) IAntsLoop {
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
		return errors.New("pool already initialized")
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
		go func() {
			p.Release()
			log.Infof("antsLoop stopped [running:%d]", p.Running())
		}()
	}
}

func (l *antsLoop) Post(job func()) {
	l.PostCtx(context.Background(), job)
}

func (l *antsLoop) PostCtx(ctx context.Context, job func()) {
	if ctx.Err() != nil {
		return
	}
	l.submit(ctx, job)
}

func (l *antsLoop) PostAndWait(job func() ([]byte, error)) ([]byte, error) {
	return l.PostAndWaitCtx(context.Background(), job)
}

func (l *antsLoop) PostAndWaitCtx(ctx context.Context, job func() ([]byte, error)) ([]byte, error) {
	resultChan := make(chan *asyncResult, 1)

	l.submit(ctx, func() {
		defer recoverPanic(resultChan)
		data, err := job()
		sendResult(ctx, resultChan, data, err)
	})

	return handleAsyncResult(ctx, resultChan)
}

func (l *antsLoop) submit(ctx context.Context, fn func()) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.pool == nil || l.pool.IsClosed() {
		l.triggerFallback(ctx, fn, "nil or closed pool")
		return
	}

	if err := l.pool.Submit(func() { safeRun(ctx, fn) }); err != nil {
		l.triggerFallback(ctx, fn, err.Error())
	}
}

func (l *antsLoop) triggerFallback(ctx context.Context, fn func(), reason string) {
	log.Warnf("Using fallback. reason:%s", reason)
	l.fallback(ctx, fn)
}

func safeRun(ctx context.Context, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("recovered panic: %v\n%s", r, debug.Stack())
		}
	}()
	if ctx.Err() == nil {
		fn()
	}
}

func recoverPanic(ch chan<- *asyncResult) {
	if r := recover(); r != nil {
		ch <- &asyncResult{nil, fmt.Errorf("panic: %v", r)}
	}
}

func sendResult(ctx context.Context, ch chan<- *asyncResult, data []byte, err error) {
	res := &asyncResult{data, err}
	select {
	case ch <- res:
	case <-ctx.Done():
	}
}

func handleAsyncResult(ctx context.Context, ch <-chan *asyncResult) ([]byte, error) {
	select {
	case res := <-ch:
		return res.data, res.err
	case <-ctx.Done():
		select {
		case res := <-ch:
			return res.data, res.err
		default:
			return nil, fmt.Errorf("operation canceled: %w", ctx.Err())
		}
	}
}
