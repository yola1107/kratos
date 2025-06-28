package work

import (
	"context"
	"time"
)

/*
	任务池+定时器任务
*/

const defaultPendingNum = 100 // 默认100条任务池缓冲空间

type IWorkStore interface {
	ITaskLoop
	ITaskScheduler
}

type workStore struct {
	ctx   context.Context
	loop  ITaskLoop
	timer ITaskScheduler
}

func NewWorkStore(ctx context.Context, pendingNum ...int) IWorkStore {
	size := defaultPendingNum
	if len(pendingNum) > 0 && pendingNum[0] > 0 {
		size = pendingNum[0]
	}
	l := NewAntsLoop(size)
	t := NewTaskScheduler(l, ctx)
	return &workStore{
		ctx:   ctx,
		loop:  l,
		timer: t,
	}
}

/*
	任务池
*/

func (w *workStore) Start() error {
	return w.loop.Start()
}

func (w *workStore) Stop() {
	w.timer.CancelAll()
	w.timer.Shutdown()
	w.loop.Stop()
}

func (w *workStore) Status() LoopStatus {
	return w.loop.Status()
}

func (w *workStore) Post(job func()) {
	w.loop.Post(job)
}

func (w *workStore) PostCtx(ctx context.Context, job func()) {
	w.loop.PostCtx(ctx, job)
}

func (w *workStore) PostAndWait(job func() ([]byte, error)) ([]byte, error) {
	return w.loop.PostAndWait(job)
}

func (w *workStore) PostAndWaitCtx(ctx context.Context, job func() ([]byte, error)) ([]byte, error) {
	return w.loop.PostAndWaitCtx(ctx, job)
}

/*
	定时器相关
*/

// Len .
func (w *workStore) Len() int {
	return w.timer.Len()
}

// Running .
func (w *workStore) Running() int32 {
	return w.timer.Running()
}

// Once .
func (w *workStore) Once(duration time.Duration, f func()) int64 {
	return w.timer.Once(duration, f)
}

// func (w *workStore) Schedule(at time.Time, f func()) int64 {
// 	return w.timer.Schedule(at, f)
// }

func (w *workStore) Forever(interval time.Duration, f func()) int64 {
	return w.timer.Forever(interval, f)
}

func (w *workStore) ForeverNow(interval time.Duration, f func()) int64 {
	return w.timer.ForeverNow(interval, f)
}

// func (w *workStore) ForeverTime(durFirst, durRepeat time.Duration, f func()) int64 {
// 	return w.timer.ForeverTime(durFirst, durRepeat, f)
// }

func (w *workStore) Cancel(taskID int64) {
	w.timer.Cancel(taskID)
}

func (w *workStore) CancelAll() {
	w.timer.CancelAll()
}

func (w *workStore) Shutdown() {
	w.timer.Shutdown()
}
