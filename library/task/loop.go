package task

import (
	"runtime/debug"

	"github.com/yola1107/kratos/v2/log"
)

/*
	任务池 job pool
*/

type Loop struct {
	jobs   chan func()
	toggle chan byte
}

func RecoverFromError(cb func()) {
	if e := recover(); e != nil {
		log.Error("Recover => %s:%s\n", e, debug.Stack())
		if cb != nil {
			cb()
		}
	}
}

// NewLoop 创建一个Loop队列，max为队列最大任务数量长度
func NewLoop(jobsCnt int) *Loop {
	return &Loop{
		jobs:   make(chan func(), jobsCnt),
		toggle: make(chan byte),
	}
}
func (lp *Loop) Start() {
	log.Infof("loop start ..")
	go func() {
		defer RecoverFromError(func() {
			lp.Start()
		})
		for {
			select {
			case <-lp.toggle:
				log.Info("Loop routine stop.")
				return
			case job := <-lp.jobs:
				job()
			}
		}
	}()
}
func (lp *Loop) Stop() {
	go func() {
		lp.toggle <- 1
	}()
}

func (lp *Loop) Jobs() int {
	return len(lp.jobs)
}

func (lp *Loop) Post(job func()) {
	go func() {
		lp.jobs <- job
	}()
}

func (lp *Loop) PostAndWait(job func() interface{}) interface{} {
	ch := make(chan interface{})
	go func() {
		lp.jobs <- func() {
			ch <- job()
		}
	}()
	return <-ch
}
