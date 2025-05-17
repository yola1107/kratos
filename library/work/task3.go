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

type ILoop3 interface {
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

func NewAntsLoop(size int) ILoop3 {
	return &antsLoop{size: size}
}

func (l *antsLoop) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.pool != nil {
		return errors.New("loop already started")
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
			log.Infof("antsLoop stopped. (async)")
		}()
	}
}

func (l *antsLoop) Post(job func()) {
	l.PostCtx(context.Background(), job)
}

func (l *antsLoop) PostCtx(ctx context.Context, job func()) {
	l.submit(ctx, func() { job() })
}

func (l *antsLoop) PostAndWait(job func() ([]byte, error)) ([]byte, error) {
	return l.PostAndWaitCtx(context.Background(), job)
}

func (l *antsLoop) PostAndWaitCtx(ctx context.Context, job func() ([]byte, error)) ([]byte, error) {
	result := make(chan struct {
		data []byte
		err  error
	}, 1)

	l.submit(ctx, func() {
		defer l.recoverWith(func(e any) {
			result <- struct {
				data []byte
				err  error
			}{nil, fmt.Errorf("panic: %v", e)}
		})
		data, err := job()
		select {
		case result <- struct {
			data []byte
			err  error
		}{data, err}:
		case <-ctx.Done():
			log.Warnf("PostAndWaitCtx: context canceled before result return: %v", ctx.Err())
		}
	})

	select {
	case res := <-result:
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
		log.Warnf("antsLoop not running, fallback to direct execution")
		go l.safeRun(ctx, fn)
		return
	}

	select {
	case <-ctx.Done():
		log.Warnf("submit canceled before run: %v", ctx.Err())
		return
	default:
	}

	if err := pool.Submit(func() {
		select {
		case <-ctx.Done():
			log.Warnf("submit canceled before execution: %v", ctx.Err())
		default:
			l.safeRun(ctx, fn)
		}
	}); err != nil {
		log.Errorf("submit failed: %v, fallback to direct", err)
		go l.safeRun(ctx, fn)
	}
}

func (l *antsLoop) safeRun(ctx context.Context, fn func()) {
	defer l.recoverWith(nil)
	select {
	case <-ctx.Done():
		log.Warnf("job canceled before execution: %v", ctx.Err())
	default:
		fn()
	}
}

func (l *antsLoop) recoverWith(cb func(any)) {
	if r := recover(); r != nil {
		log.Errorf("Recover: %v\n%s", r, debug.Stack())
		if cb != nil {
			cb(r)
		}
	}
}
