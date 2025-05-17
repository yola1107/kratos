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

//type ILoop interface {
//	Start() error
//	Stop()
//	Post(job func())
//	PostCtx(ctx context.Context, job func())
//	PostAndWait(job func() ([]byte, error)) ([]byte, error)
//	PostAndWaitCtx(ctx context.Context, job func() ([]byte, error)) ([]byte, error)
//}

type AntsLoop struct {
	pool *ants.Pool
	mu   sync.RWMutex
	size int
}

func NewAntsLoop(size int) *AntsLoop {
	return &AntsLoop{
		size: size,
	}
}

func (l *AntsLoop) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.pool != nil {
		return errors.New("loop already started")
	}

	p, err := ants.NewPool(l.size, ants.WithPanicHandler(func(i interface{}) {
		log.Errorf("task panic: %v\n%s", i, debug.Stack())
	}))
	if err != nil {
		return err
	}
	l.pool = p
	log.Infof("loop start")
	return nil
}

func (l *AntsLoop) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.pool != nil {
		pool := l.pool
		l.pool = nil
		go func() {
			pool.Release()
			log.Infof("loop stopped (async)")
		}()
	}
}

func (l *AntsLoop) IsRunning() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.pool != nil && !l.pool.IsClosed()
}

func (l *AntsLoop) Post(job func()) {
	l.PostCtx(context.Background(), job)
}

func (l *AntsLoop) PostCtx(ctx context.Context, job func()) {
	l.mu.RLock()
	pool := l.pool
	l.mu.RUnlock()

	if pool == nil {
		log.Warnf("loop not running, fallback to direct execution")
		defer RecoverFromError(nil)
		job()
		return
	}

	select {
	case <-ctx.Done():
		log.Warnf("PostCtx canceled before submit: %v", ctx.Err())
		return
	default:
	}

	err := pool.Submit(func() {
		defer RecoverFromError(nil)
		select {
		case <-ctx.Done():
			log.Warnf("PostCtx canceled before job run: %v", ctx.Err())
			return
		default:
			job()
		}
	})

	if err != nil {
		log.Errorf("submit failed: %v", err)
		go func() {
			defer RecoverFromError(nil)
			select {
			case <-ctx.Done():
				log.Warnf("PostCtx fallback canceled: %v", ctx.Err())
				return
			default:
				job()
			}
		}()
	}
}

func (l *AntsLoop) PostAndWait(job func() ([]byte, error)) ([]byte, error) {
	return l.PostAndWaitCtx(context.Background(), job)
}
func (l *AntsLoop) PostAndWaitCtx(ctx context.Context, job func() ([]byte, error)) ([]byte, error) {
	l.mu.RLock()
	pool := l.pool
	l.mu.RUnlock()

	if pool == nil {
		log.Warnf("loop not running, fallback to direct execution")
		defer RecoverFromError(nil)
		return job()
	}

	type jobResult struct {
		data []byte
		err  error
	}
	result := make(chan jobResult, 1)

	err := pool.Submit(func() {
		defer RecoverFromError(func(e interface{}) {
			select {
			case result <- jobResult{nil, fmt.Errorf("panic: %v", e)}:
			default:
			}
		})
		data, err := job()
		select {
		case result <- jobResult{data, err}:
		case <-ctx.Done():
			log.Warnf("PostAndWaitCtx: context done before sending result: %v", ctx.Err())
		}
	})

	if err != nil {
		log.Errorf("submit failed: %v, fallback to direct execution", err)
		return job()
	}

	select {
	case res := <-result:
		return res.data, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func RecoverFromError(cb func(interface{})) {
	if e := recover(); e != nil {
		log.Errorf("Recover => %v:%s\n", e, debug.Stack())
		if cb != nil {
			cb(e)
		}
	}
}
