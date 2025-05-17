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

type IAntsLoop interface {
	Start() error
	Stop()
	Post(job func())
	PostCtx(ctx context.Context, job func())
	PostAndWait(job func() ([]byte, error)) ([]byte, error)
	PostAndWaitCtx(ctx context.Context, job func() ([]byte, error)) ([]byte, error)
}

type Option func(*antsLoop)

func WithFallback(fallback func(ctx context.Context, fn func())) Option {
	return func(l *antsLoop) {
		l.fallback = fallback
	}
}

type antsLoop struct {
	mu       sync.RWMutex
	pool     *ants.Pool
	size     int
	fallback func(ctx context.Context, fn func())
}

func NewAntsLoop(size int, opts ...Option) IAntsLoop {
	l := &antsLoop{
		size:     size,
		fallback: defaultFallback, // 设置默认回退
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

func defaultFallback(ctx context.Context, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("fallback panic: %v\n%s", r, debug.Stack())
			}
		}()
		fn()
	}()
}

func (l *antsLoop) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.pool != nil {
		return errors.New("antsLoop: already started")
	}

	pool, err := ants.NewPool(l.size, ants.WithPanicHandler(func(r any) {
		log.Errorf("panic in task: %v\n%s", r, debug.Stack())
	}))
	if err != nil {
		return fmt.Errorf("create pool failed: %w", err)
	}

	l.pool = pool
	log.Infof("antsLoop started (pool_size=%d)", l.size)
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
			log.Infof("antsLoop stopped (async)")
		}()
	}
}

func (l *antsLoop) Post(job func()) {
	l.PostCtx(context.Background(), job)
}

func (l *antsLoop) PostCtx(ctx context.Context, job func()) {
	l.submit(ctx, job)
}

func (l *antsLoop) PostAndWait(job func() ([]byte, error)) ([]byte, error) {
	return l.PostAndWaitCtx(context.Background(), job)
}

func (l *antsLoop) PostAndWaitCtx(ctx context.Context, job func() ([]byte, error)) ([]byte, error) {
	resultChan := make(chan asyncResult, 1)

	l.submit(ctx, func() {
		defer l.recover(func(r any) {
			resultChan <- asyncResult{nil, fmt.Errorf("panic: %v", r)}
		})

		data, err := job()
		select {
		case resultChan <- asyncResult{data, err}:
		case <-ctx.Done():
			log.Warn("job result abandoned due to context cancellation")
		}
	})

	select {
	case res := <-resultChan:
		return res.data, res.err
	case <-ctx.Done():
		select {
		case res := <-resultChan:
			return res.data, res.err
		default:
			return nil, ctx.Err()
		}
	}
}

func (l *antsLoop) submit(ctx context.Context, fn func()) {
	select {
	case <-ctx.Done():
		log.Warnf("submit aborted: %v", ctx.Err())
		return
	default:
	}

	l.mu.RLock()
	pool := l.pool
	l.mu.RUnlock()

	if pool == nil || pool.IsClosed() {
		log.Warn("antsLoop not active, using fallback")
		l.runFallback(ctx, fn)
		return
	}

	if err := pool.Submit(func() { l.safeRun(ctx, fn) }); err != nil {
		log.Errorf("submit failed: %v, using fallback", err)
		l.runFallback(ctx, fn)
	}
}

func (l *antsLoop) safeRun(ctx context.Context, fn func()) {
	defer l.recover(nil)

	select {
	case <-ctx.Done():
		log.Warnf("job execution aborted: %v", ctx.Err())
	default:
		fn()
	}
}

func (l *antsLoop) runFallback(ctx context.Context, fn func()) {
	if l.fallback != nil {
		l.fallback(ctx, fn)
	} else {
		defaultFallback(ctx, fn)
	}
}

func (l *antsLoop) recover(cb func(any)) {
	if r := recover(); r != nil {
		log.Errorf("recovered panic: %v\n%s", r, debug.Stack())
		if cb != nil {
			cb(r)
		}
	}
}
