package work

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"

	"github.com/yola1107/kratos/v2/log"
)

var resultChanPool = sync.Pool{
	New: func() any {
		return make(chan *asyncResult, 1)
	},
}

var asyncResultPool = sync.Pool{
	New: func() any {
		return new(asyncResult)
	},
}

// 定义结果类型
type asyncResult struct {
	data []byte
	err  error
}

// LoopStatus 定义当前池状态结构体
type LoopStatus struct {
	Capacity int // 池最大容量
	Running  int // 当前运行中协程数
	Free     int // 空闲协程数（Capacity - Running）
}

// ITaskLoop 协程池管理接口
type ITaskLoop interface {
	Start() error
	Stop()
	Status() LoopStatus
	Post(job func())
	PostCtx(ctx context.Context, job func())
	PostAndWait(job func() ([]byte, error)) ([]byte, error)
	PostAndWaitCtx(ctx context.Context, job func() ([]byte, error)) ([]byte, error)
}

type Option func(*antsLoop)

// WithFallback 自定义任务提交失败处理策略
func WithFallback(fallback func(ctx context.Context, fn func())) Option {
	return func(l *antsLoop) {
		l.fallback = fallback
	}
}

// WithPoolOptions 自定义ants池选项
func WithPoolOptions(opts ...ants.Option) Option {
	return func(l *antsLoop) {
		l.poolOptions = append(l.poolOptions, opts...)
	}
}

type antsLoop struct {
	mu          sync.RWMutex
	pool        *ants.Pool
	size        int
	fallback    func(context.Context, func())
	poolOptions []ants.Option
}

// NewAntsLoop 创建协程池实例
func NewAntsLoop(size int, opts ...Option) ITaskLoop {
	l := &antsLoop{
		size: size,
		fallback: func(ctx context.Context, fn func()) {
			go safeRun(ctx, fn)
		},
		poolOptions: []ants.Option{
			ants.WithExpiryDuration(60 * time.Second), // 每60s清理一次闲置 worker
			// ants.WithPreAlloc(true),                 // 预分配容量，避免 runtime 扩容内存
			// ants.WithNonblocking(false),  // 非阻塞提交，任务满时立即报错（否则阻塞） 默认阻塞模式:true
			// ants.WithMaxBlockingTasks(0), // 最大阻塞任务数（非阻塞模式下可设为0）
		},
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

func (l *antsLoop) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.pool != nil {
		log.Warnf("antsLoop already started.")
		return nil
	}

	// 创建协程池
	pool, err := ants.NewPool(l.size, l.poolOptions...)
	if err != nil {
		return fmt.Errorf("pool init failed: %w", err)
	}

	l.pool = pool
	log.Infof("antsLoop start... [size:%d]", l.size)
	return nil
}

func (l *antsLoop) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.pool != nil {
		p := l.pool
		l.pool = nil
		p.Release()
		log.Infof("antsLoop stopping [running:%d]", p.Running())
	}
}

// Status 获取当前池状态
func (l *antsLoop) Status() LoopStatus {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.pool == nil {
		return LoopStatus{}
	}

	capacity := l.pool.Cap()
	running := l.pool.Running()
	free := capacity - running
	if free < 0 {
		free = 0
	}

	return LoopStatus{
		Capacity: capacity,
		Running:  running,
		Free:     free,
	}
}

func (l *antsLoop) Post(job func()) {
	l.PostCtx(context.Background(), job)
}

func (l *antsLoop) PostCtx(ctx context.Context, job func()) {
	if ctx.Err() == nil {
		l.submit(ctx, job)
	}
}

func (l *antsLoop) PostAndWait(job func() ([]byte, error)) ([]byte, error) {
	return l.PostAndWaitCtx(context.Background(), job)
}

func (l *antsLoop) PostAndWaitCtx(ctx context.Context, job func() ([]byte, error)) ([]byte, error) {
	ch := resultChanPool.Get().(chan *asyncResult)

	// 清理通道并归还池中
	defer func() {
		select {
		case <-ch: // drain
		default:
		}
		resultChanPool.Put(ch)
	}()

	l.submit(ctx, func() {
		defer RecoverFromError(func(e any) {
			res := asyncResultPool.Get().(*asyncResult)
			res.data = nil
			res.err = fmt.Errorf("panic: %v", e)

			select {
			case ch <- res:
			default:
			}
		})

		data, err := job()
		res := asyncResultPool.Get().(*asyncResult)
		res.data = data
		res.err = err

		select {
		case ch <- res:
		case <-ctx.Done():
		}
	})

	select {
	case res := <-ch:
		defer asyncResultPool.Put(res)
		return res.data, res.err
	case <-ctx.Done():
		select {
		case res := <-ch:
			defer asyncResultPool.Put(res)
			return res.data, res.err
		default:
			// 确保job被取消的信息能发送出去, 防止调用方一直阻塞等待接收job的返回结果
			return nil, fmt.Errorf("canceled: %w", ctx.Err())
		}
	}
}

func (l *antsLoop) submit(ctx context.Context, fn func()) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.pool == nil || l.pool.IsClosed() {
		l.triggerFallback(ctx, fn, "loop not started or loop is closed.")
		return
	}

	if err := l.pool.Submit(func() { safeRun(ctx, fn) }); err != nil {
		l.triggerFallback(ctx, fn, err.Error())
	}
}

func (l *antsLoop) triggerFallback(ctx context.Context, fn func(), reason string) {
	log.Warnf("ansloop fallback. reason=%s", reason)
	l.fallback(ctx, fn)
}

func safeRun(ctx context.Context, fn func()) {
	defer RecoverFromError(nil)
	if ctx.Err() == nil {
		fn()
	}
}
