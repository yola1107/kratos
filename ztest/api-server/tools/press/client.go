package press

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
)

type Runner struct {
	conf   *Bootstrap
	logger *zap.Logger

	loop   work.ITaskLoop      // 任务循环
	timer  work.ITaskScheduler // 定时任务
	ctx    context.Context
	cancel context.CancelFunc

	users sync.Map
	count atomic.Int32
}

func NewRunner(conf *Bootstrap, logger *zap.Logger) *Runner {
	ctx, cancel := context.WithCancel(context.Background())
	loop := work.NewAntsLoop(work.WithSize(100))
	timer := work.NewTaskScheduler(
		work.WithContext(ctx),
		work.WithExecutor(loop),
	)
	return &Runner{
		loop:   loop,
		timer:  timer,
		conf:   conf,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (r *Runner) Start() {
	if err := r.loop.Start(); err != nil {
		panic(err)
	}
	log.Infof("start press runner, conf:%s", r.conf)
}

func (r *Runner) Stop() {
	r.cancel()
	r.timer.Stop()
	r.loop.Stop()
	log.Infof("stop success")
}
