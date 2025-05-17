package work

import (
	"sync"
	"sync/atomic"

	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/log"
)

/*
	任务池 job pool
*/

type ILoop interface {
	Start()
	Stop()
	Jobs() int
	Post(job func())
	PostAndWait(job func() ([]byte, error)) ([]byte, error)
}

type taskBuffer struct {
	jobs    chan func()
	toggle  chan byte
	once    sync.Once
	stopped atomic.Bool
}

// NewTaskBuffer 创建一个Loop队列，max为队列最大任务数量长度
func NewTaskBuffer(jobsCnt int) ILoop {
	return &taskBuffer{
		jobs:   make(chan func(), jobsCnt),
		toggle: make(chan byte),
	}
}

func (lp *taskBuffer) Start() {
	log.Infof("loop start ..")
	go func() {
		defer ext.RecoverFromError(func() {
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

func (lp *taskBuffer) Stop() {
	lp.once.Do(
		func() { close(lp.toggle) },
	)
}

func (lp *taskBuffer) Jobs() int {
	return len(lp.jobs)
}

func (lp *taskBuffer) Post(job func()) {
	go func() {
		lp.jobs <- job
	}()
}

func (lp *taskBuffer) PostAndWait(job func() ([]byte, error)) ([]byte, error) {
	type waitResult struct {
		data []byte
		err  error
	}

	result := make(chan waitResult, 1)

	select {
	case lp.jobs <- func() {
		data, err := job()
		result <- waitResult{data: data, err: err}
	}:
		res := <-result
		return res.data, res.err
	default:
		log.Warnf("loop queue full. capacity(%d)", len(lp.jobs))
		return job()
	}
}
