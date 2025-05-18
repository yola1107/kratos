package work

import (
	"sync"
	"sync/atomic"

	"github.com/yola1107/kratos/v2/log"
)

/*
	任务池 job pool
	注意: 当调用PostAndWait/PostAndWaitAny时,job内部如果发生panic,调用方会拿不到job返回的结果而一直阻塞等待
*/

type ILoop interface {
	Start()
	Stop()
	Jobs() int
	Post(job func())
	PostAndWait(job func() ([]byte, error)) ([]byte, error)
	PostAndWaitAny(job func() any) any
}

type Loop struct {
	jobs    chan func()
	toggle  chan byte
	once    sync.Once
	stopped atomic.Bool
}

// NewLoop 创建一个Loop队列，max为队列最大任务数量长度
func NewLoop(jobsCnt int) ILoop {
	return &Loop{
		jobs:   make(chan func(), jobsCnt),
		toggle: make(chan byte),
	}
}

func (lp *Loop) Start() {
	log.Infof("loop start ..")
	go func() {
		defer recoverFromError(func(e any) {
			lp.Start()
		})
		for {
			select {
			case <-lp.toggle:
				lp.stopped.Store(true)
				log.Infof("loop routine stop. Remaining(%d)", lp.Jobs())
				return
			case job := <-lp.jobs:
				job()
			}
		}
	}()
}

func (lp *Loop) Stop() {
	lp.once.Do(
		func() { close(lp.toggle) },
	)
}

func (lp *Loop) Jobs() int {
	return len(lp.jobs)
}

func (lp *Loop) Post(job func()) {
	go func() {
		lp.jobs <- job
	}()
}

func (lp *Loop) PostAndWait(job func() ([]byte, error)) ([]byte, error) {
	ch := make(chan []byte)
	var err error
	go func() {
		lp.jobs <- func() {
			rsp, rerr := job()
			err = rerr
			ch <- rsp
		}
	}()
	rsp := <-ch
	return rsp, err
}

func (lp *Loop) PostAndWaitAny(job func() any) any {
	ch := make(chan any)
	go func() {
		lp.jobs <- func() {
			ch <- job()
		}
	}()
	return <-ch
}
