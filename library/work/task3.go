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

type IAntsLoop interface {
	Start() error
	Stop()
	Post(job func())
	PostCtx(ctx context.Context, job func())
	PostAndWait(job func() ([]byte, error)) ([]byte, error)
	PostAndWaitCtx(ctx context.Context, job func() ([]byte, error)) ([]byte, error)
}

type antsLoop struct {
	mu   sync.RWMutex
	pool *ants.Pool
	size int
}

func NewAntsLoop(size int) IAntsLoop {
	return &antsLoop{size: size}
}

func (l *antsLoop) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.pool != nil {
		return errors.New("antsLoop already started")
	}

	pool, err := ants.NewPool(l.size, ants.WithPanicHandler(func(i any) {
		log.Errorf("panic in task: %v\n%s", i, debug.Stack())
	}))
	if err != nil {
		return err
	}

	l.pool = pool
	log.Infof("antsLoop started")
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
	resultCh := make(chan struct {
		data []byte
		err  error
	}, 1)

	l.submit(ctx, func() {
		defer l.recover(func(r any) {
			resultCh <- struct {
				data []byte
				err  error
			}{nil, fmt.Errorf("panic: %v", r)}
		})

		data, err := job()
		select {
		case resultCh <- struct {
			data []byte
			err  error
		}{data, err}:
		case <-ctx.Done():
			log.Warnf("PostAndWaitCtx: context canceled before sending result: %v", ctx.Err())
		}
	})

	select {
	case res := <-resultCh:
		return res.data, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (l *antsLoop) submit(ctx context.Context, fn func()) {
	l.mu.RLock()
	pool := l.pool
	l.mu.RUnlock()

	if pool == nil || pool.IsClosed() {
		log.Warnf("antsLoop not running, execute directly")
		go l.safeRun(ctx, fn)
		return
	}

	if err := pool.Submit(func() {
		l.safeRun(ctx, fn)
	}); err != nil {
		log.Errorf("submit failed: %v, execute directly", err)
		go l.safeRun(ctx, fn)
	}
}

func (l *antsLoop) safeRun(ctx context.Context, fn func()) {
	defer l.recover(nil)
	select {
	case <-ctx.Done():
		log.Warnf("job canceled before execution: %v", ctx.Err())
	default:
		fn()
	}
}

func (l *antsLoop) recover(cb func(any)) {
	if r := recover(); r != nil {
		log.Errorf("recovered from panic: %v\n%s", r, debug.Stack())
		if cb != nil {
			cb(r)
		}
	}
}
