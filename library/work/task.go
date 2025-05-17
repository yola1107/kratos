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

/*
	任务池 job pool
*/

type ILoop interface {
	Start() error
	Stop()
	Post(job func())
	PostCtx(ctx context.Context, job func())
	PostAndWait(job func() ([]byte, error)) ([]byte, error)
	PostAndWaitCtx(ctx context.Context, job func() ([]byte, error)) ([]byte, error)
}

type AntsLoop struct {
	pool *ants.Pool
	mu   sync.RWMutex
	size int
}

func NewAntsLoop(size int) ILoop {
	return &AntsLoop{
		size: size,
	}
}

func (al *AntsLoop) Start() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.pool != nil {
		return errors.New("loop already started")
	}

	p, err := ants.NewPool(al.size, ants.WithPanicHandler(func(i interface{}) {
		log.Infof("task panic: %v\n%s", i, debug.Stack())
	}))
	if err != nil {
		return err
	}
	al.pool = p
	log.Infof("loop start")
	return nil
}

func (al *AntsLoop) Stop() {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.pool != nil {
		//根据ants文档，Release会关闭池并等待所有任务完成，所以这里没问题。
		al.pool.Release()
		al.pool = nil
		log.Infof("loop stopped")
	}
}

func (al *AntsLoop) Post(job func()) {
	al.PostCtx(context.Background(), job)
}

func (al *AntsLoop) PostCtx(ctx context.Context, job func()) {
	al.mu.RLock()
	pool := al.pool
	al.mu.RUnlock()

	if pool == nil {
		log.Infof("loop not running")
		return
	}

	// ctx 先行检查
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

func (al *AntsLoop) PostAndWait(job func() ([]byte, error)) ([]byte, error) {
	return al.PostAndWaitCtx(context.Background(), job)
}

func (al *AntsLoop) PostAndWaitCtx(ctx context.Context, job func() ([]byte, error)) ([]byte, error) {
	al.mu.RLock()
	pool := al.pool
	al.mu.RUnlock()

	if pool == nil {
		return nil, errors.New("loop not running")
	}

	type jobResult struct {
		data []byte
		err  error
	}

	result := make(chan jobResult, 1)

	err := pool.Submit(func() {
		defer RecoverFromError(func() {
			select {
			case result <- jobResult{nil, fmt.Errorf("PostAndWait panic")}:
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
		log.Errorf("submit failed: %v", err)
		return job()
	}

	select {
	case res := <-result:
		return res.data, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func RecoverFromError(cb func()) {
	if e := recover(); e != nil {
		log.Errorf("Recover => %v:%s\n", e, debug.Stack())
		if cb != nil {
			cb()
		}
	}
}
