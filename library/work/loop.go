package work

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/yola1107/kratos/v2/log"
)

const defaultPendingNum = 100 // 默认协程池大小

// asyncResult封装任务结果
type asyncResult struct {
	data []byte
	err  error
}

// LoopMonitor 协程池当前状态
type LoopMonitor struct {
	Capacity int // 池最大容量
	Running  int // 当前运行协程数
	Free     int // 空闲协程数（Capacity - Running）
}

// ITaskLoop 定义协程池接口
type ITaskLoop interface {
	Start() error
	Stop()
	Monitor() LoopMonitor
	Post(job func())
	PostCtx(ctx context.Context, job func())
	PostAndWait(job func() ([]byte, error)) ([]byte, error)
	PostAndWaitCtx(ctx context.Context, job func() ([]byte, error)) ([]byte, error)
}

type Option func(*antsLoop)

// WithSize 设置池大小
func WithSize(size int) Option {
	return func(l *antsLoop) {
		if size > 0 {
			l.size = size
		}
	}
}

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

// NewAntsLoop 创建协程池实例，支持传入参数
func NewAntsLoop(opts ...Option) ITaskLoop {
	l := &antsLoop{
		size: defaultPendingNum,
		fallback: func(ctx context.Context, fn func()) {
			go safeRun(ctx, fn)
		},
		poolOptions: []ants.Option{
			ants.WithExpiryDuration(120 * time.Second), // 每120s清理一次闲置 worker
			// ants.WithPreAlloc(true),                    // 预分配容量，避免 runtime 扩容内存
			// ants.WithNonblocking(false),                // false:默认阻塞模式 true:非阻塞提交，任务满时立即报错
			// ants.WithMaxBlockingTasks(0),               // 最大阻塞任务数（非阻塞模式下可设为0）
		},
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Start 启动池，初始化 ants.Pool
func (l *antsLoop) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.pool != nil {
		log.Warnf("antsLoop already started.")
		return nil
	}

	pool, err := ants.NewPool(l.size, l.poolOptions...)
	if err != nil {
		return fmt.Errorf("pool init failed: %w", err)
	}

	l.pool = pool
	log.Infof("antsLoop start... [size:%d]", l.size)
	return nil
}

// Stop 停止池，释放资源
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

// Monitor 返回当前池状态
func (l *antsLoop) Monitor() LoopMonitor {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.pool == nil {
		return LoopMonitor{}
	}

	capacity := l.pool.Cap()
	running := l.pool.Running()
	free := capacity - running
	if free < 0 {
		free = 0
	}

	return LoopMonitor{
		Capacity: capacity,
		Running:  running,
		Free:     free,
	}
}

// Post 提交无返回任务，使用 background context
func (l *antsLoop) Post(job func()) {
	l.PostCtx(context.Background(), job)
}

// PostCtx 提交无返回任务，携带上下文
func (l *antsLoop) PostCtx(ctx context.Context, job func()) {
	if ctx.Err() == nil {
		l.submit(ctx, job)
	}
}

// PostAndWait 提交有返回结果任务，阻塞等待结果，使用 background context
func (l *antsLoop) PostAndWait(job func() ([]byte, error)) ([]byte, error) {
	return l.PostAndWaitCtx(context.Background(), job)
}

// PostAndWaitCtx 提交有返回结果任务，携带上下文，阻塞等待或超时取消
func (l *antsLoop) PostAndWaitCtx(ctx context.Context, job func() ([]byte, error)) ([]byte, error) {
	ch := make(chan *asyncResult, 1)

	l.submit(ctx, func() {
		defer RecoverFromError(func(e any) {
			select {
			case ch <- &asyncResult{nil, fmt.Errorf("panic: %v", e)}:
			default:
			}
		})
		data, err := job()
		select {
		case ch <- &asyncResult{data, err}:
		case <-ctx.Done():
		}
	})

	select {
	case res := <-ch:
		return res.data, res.err
	case <-ctx.Done():
		select {
		case res := <-ch:
			return res.data, res.err
		default:
			// 确保job被取消的信息能发送出去, 防止调用方一直阻塞等待接收job的返回结果
			return nil, fmt.Errorf("canceled: %w", ctx.Err())
		}
	}
}

// submit 负责任务提交和fallback处理，保证安全调用
func (l *antsLoop) submit(ctx context.Context, fn func()) {
	l.mu.RLock()
	pool := l.pool
	l.mu.RUnlock()

	if pool == nil || pool.IsClosed() {
		l.triggerFallback(ctx, fn, "loop not started or loop is closed.")
		return
	}

	if err := pool.Submit(func() { safeRun(ctx, fn) }); err != nil {
		l.triggerFallback(ctx, fn, err.Error())
	}
}

func (l *antsLoop) triggerFallback(ctx context.Context, fn func(), reason string) {
	log.Warnf("antsLoop fallback. reason=%s", reason)
	l.fallback(ctx, fn)
}

// safeRun 包装任务执行，捕获panic且只在ctx未取消时执行
func safeRun(ctx context.Context, fn func()) {
	defer RecoverFromError(nil)
	if ctx.Err() == nil {
		fn()
	}
}
