package work

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/panjf2000/ants/v2"
	"github.com/yola1107/kratos/v2/log"
)

type ILoop interface {
	Start() error
	Stop()
	Post(job func())
	PostAndWait(job func() ([]byte, error)) ([]byte, error)
}

type AntsLoop struct {
	pool       *ants.Pool
	mu         sync.RWMutex
	maxWorkers int
}

func NewAntsLoop(maxWorkers int) ILoop {
	return &AntsLoop{
		maxWorkers: maxWorkers,
	}
}

func (al *AntsLoop) Start() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.pool != nil {
		return errors.New("loop already started")
	}

	p, err := ants.NewPool(al.maxWorkers, ants.WithPanicHandler(func(i interface{}) {
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
		al.pool.Release()
		al.pool = nil
		log.Infof("loop stopped")
	}
}

func (al *AntsLoop) Post(job func()) {
	al.mu.RLock()
	pool := al.pool
	al.mu.RUnlock()

	if pool == nil {
		log.Infof("loop not running")
		return
	}

	err := pool.Submit(func() {
		defer func() {
			if r := recover(); r != nil {
				log.Infof("recover from panic: %v\n%s", r, debug.Stack())
			}
		}()
		job()
	})
	if err != nil {
		log.Infof("submit failed: %v", err)
		go job()
	}
}

func (al *AntsLoop) PostAndWait(job func() ([]byte, error)) ([]byte, error) {
	al.mu.RLock()
	pool := al.pool
	al.mu.RUnlock()

	if pool == nil {
		return nil, errors.New("loop not running")
	}

	result := make(chan struct {
		data []byte
		err  error
	}, 1)

	err := pool.Submit(func() {
		defer func() {
			if r := recover(); r != nil {
				result <- struct {
					data []byte
					err  error
				}{nil, fmt.Errorf("panic: %v", r)}
			}
		}()
		data, err := job()
		result <- struct {
			data []byte
			err  error
		}{data, err}
	})
	if err != nil {
		log.Infof("submit failed: %v", err)
		return job()
	}

	select {
	case res := <-result:
		return res.data, res.err
	//case <-time.After(5 * time.Second):
	//	return nil, context.DeadlineExceeded
	default:
		return job()
	}
}
