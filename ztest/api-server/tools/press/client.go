package press

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
)

type Runner struct {
	conf   *LoadTest
	logger *zap.Logger

	loop   work.ITaskLoop      // 任务循环
	timer  work.ITaskScheduler // 定时任务
	ctx    context.Context
	cancel context.CancelFunc

	users  sync.Map
	count  atomic.Int32
	nextID atomic.Int64
}

func NewRunner(conf *LoadTest, logger *zap.Logger) *Runner {
	ctx, cancel := context.WithCancel(context.Background())
	loop := work.NewAntsLoop(work.WithSize(10000))
	timer := work.NewTaskScheduler(
		work.WithContext(ctx),
		work.WithExecutor(loop),
	)
	r := &Runner{
		loop:   loop,
		timer:  timer,
		conf:   conf,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
	r.nextID.Store(conf.Press.StartID)
	return r
}

func (r *Runner) GetTimer() work.ITaskScheduler {
	return r.timer
}

func (r *Runner) GetLoop() work.ITaskLoop {
	return r.loop
}
func (r *Runner) GetContext() context.Context {
	return r.ctx
}

func (r *Runner) GetUrl() string {
	return r.conf.Press.Url
}

func (r *Runner) Start() {
	if err := r.loop.Start(); err != nil {
		panic(err)
	}
	interval := time.Duration(r.conf.Press.Interval) * time.Millisecond
	r.timer.Forever(interval, r.Load)
	r.timer.Forever(15*time.Second, r.Release)
	r.timer.Forever(15*time.Second, r.Monitor)
	log.Infof("start client success. conf:%+v", r.conf.Press)
}

func (r *Runner) Stop() {
	r.cancel()
	r.timer.CancelAll()
	r.timer.Stop()
	r.loop.Stop()
	log.Infof("stop client success")
}

func (r *Runner) Monitor() {
	log.Infof("[monitor] <client> loop=%+v timer=%+v player={Num:%v curr:%v}",
		r.loop.Monitor(), r.timer.Monitor(), r.conf.Press.Num, r.count.Load())
}

func (r *Runner) Load() {
	conf := r.conf.Press
	if !conf.Open {
		return
	}

	toLoad := min(conf.Num-r.count.Load(), conf.Batch)
	if toLoad <= 0 {
		return
	}

	startID := conf.StartID
	idRange := int64(conf.Num * 2)
	loaded := int32(0)
	attempts := int32(0)

	for loaded < toLoad && attempts < conf.Num {
		attempts++

		id := startID + (r.nextID.Add(1) % idRange) // ID ∈ [startID, startID + 2*Num)

		if _, exists := r.users.Load(id); exists {
			continue
		}

		user, err := NewUser(id, r)
		if err != nil || user == nil {
			log.Warnf("load user err: %v", err)
			continue
		}

		r.users.Store(id, user)
		r.count.Add(1)
		loaded++
	}
}

func (r *Runner) Release() {
	r.users.Range(func(key, value interface{}) bool {
		user := value.(*User)
		if !user.IsFree() {
			return true
		}
		user.Release()
		r.users.Delete(key)
		r.count.Add(-1)

		return true
	})
}
