package alert

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go.uber.org/zap/zapcore"

	"github.com/yola1107/kratos/v2/library/log/config"
	"github.com/yola1107/kratos/v2/log"
)

const (
	maxTelegramMsgSize = 4096 - 100 // Telegram消息最大长度 4k，保留100字节余量
)

// Sender 发送接口
type Sender interface {
	Send(messages []string) error
	Close() error
}

// Alerter 报警器核心
type Alerter struct {
	zapcore.LevelEnabler
	enc    zapcore.Encoder
	fields []zapcore.Field

	conf   *config.Alert
	sender Sender
	queue  *BufferQueue[string]
	mu     sync.RWMutex
}

func NewAlerter(enabler zapcore.LevelEnabler, enc zapcore.Encoder, conf *config.Alert) *Alerter {
	if conf == nil || !conf.Enabled {
		return nil
	}

	sender, err := NewTelegramSender(conf.Telegram)
	if err != nil {
		log.Errorf("Failed to create Telegram sender: %v", err)
		return nil
	}

	a := &Alerter{
		LevelEnabler: enabler,
		enc:          enc,
		conf:         conf,
		sender:       sender,
	}

	a.queue = NewBufferQueue[string](
		conf,
		func(batch []string) {
			a.sendWithRetry(batch)
		},
	)

	return a
}

// With 添加字段
func (a *Alerter) With(fields []zapcore.Field) zapcore.Core {
	if len(fields) == 0 {
		return a
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	clone := *a
	clone.enc = a.enc.Clone()
	clone.fields = make([]zapcore.Field, len(a.fields), len(a.fields)+len(fields))
	copy(clone.fields, a.fields)
	clone.fields = append(clone.fields, fields...)

	return &clone
}

// Check 检查日志级别
func (a *Alerter) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if a.Enabled(ent.Level) {
		return ce.AddCore(ent, a)
	}
	return ce
}

func (a *Alerter) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	entryBuf, err := a.enc.EncodeEntry(ent, append(a.fields, fields...))
	if err != nil {
		return fmt.Errorf("failed to encode entry: %w", err)
	}

	msg := truncateMessage(entryBuf.String(), maxTelegramMsgSize)
	return a.queue.Push(msg)
}

func (a *Alerter) Sync() error { return nil }

func (a *Alerter) Close() error {
	a.queue.Close()
	if err := a.sender.Close(); err != nil {
		return fmt.Errorf("failed to close sender: %w", err)
	}
	return nil
}

func (a *Alerter) sendWithRetry(msgs []string) {
	if len(msgs) == 0 {
		return
	}

	var lastErr error
	for i := 0; i < a.conf.MaxRetries; i++ {
		if err := a.sender.Send(msgs); err == nil {
			return
		} else {
			lastErr = err
		}

		if i < a.conf.MaxRetries-1 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	log.Errorf("Failed to send alert batch after %d retries: %v", a.conf.MaxRetries, lastErr)
}

// BufferQueue 实现缓冲批处理队列
type BufferQueue[T any] struct {
	ch       chan T
	stopChan chan struct{}
	wg       sync.WaitGroup

	batchSize int
	maxWait   time.Duration
	handler   func([]T)

	mu         sync.Mutex
	buffer     []T
	bufferSize int
	timer      *time.Timer
	pool       sync.Pool
}

func NewBufferQueue[T any](conf *config.Alert, handler func([]T)) *BufferQueue[T] {
	q := &BufferQueue[T]{
		ch:        make(chan T, conf.QueueSize),
		stopChan:  make(chan struct{}),
		batchSize: conf.MaxBatchCnt,
		maxWait:   conf.MaxInterval,
		handler:   handler,
		timer:     time.NewTimer(conf.MaxInterval),
		pool: sync.Pool{
			New: func() interface{} {
				return make([]T, 0, conf.MaxBatchCnt)
			},
		},
	}

	q.wg.Add(1)
	go q.process()

	return q
}

func (q *BufferQueue[T]) Push(item T) error {
	select {
	case q.ch <- item:
		return nil
	case <-q.stopChan:
		return context.Canceled
	default:
		return fmt.Errorf("queue full (capacity=%d)", cap(q.ch))
	}
}

func (q *BufferQueue[T]) Close() {
	close(q.stopChan)
	q.wg.Wait()
}

func (q *BufferQueue[T]) process() {
	defer q.wg.Done()
	defer q.timer.Stop()

	for {
		select {
		case item := <-q.ch:
			q.mu.Lock()
			q.buffer = append(q.buffer, item)
			shouldFlush := len(q.buffer) >= q.batchSize //|| (len(q.buffer) > 0 && len(item)+len(q.buffer[0]) > maxTelegramMsgSize)

			q.mu.Unlock()

			if shouldFlush {
				q.flush()
			} else {
				q.resetTimer()
			}

		case <-q.timer.C:
			q.flush()

		case <-q.stopChan:
			q.drain()
			return
		}
	}
}

func (q *BufferQueue[T]) flush() {
	q.mu.Lock()
	if len(q.buffer) == 0 {
		q.mu.Unlock()
		return
	}

	// 从内存池获取切片
	batch := q.pool.Get().([]T)[:0]
	batch = append(batch, q.buffer...)
	q.buffer = q.buffer[:0]
	q.mu.Unlock()

	q.handler(batch)

	// 归还切片到内存池
	q.pool.Put(batch[:0])
	q.resetTimer()
}

func (q *BufferQueue[T]) drain() {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 处理剩余消息
	for {
		select {
		case item := <-q.ch:
			q.buffer = append(q.buffer, item)
			if len(q.buffer) >= q.batchSize {
				batch := q.pool.Get().([]T)[:0]
				batch = append(batch, q.buffer...)
				q.buffer = q.buffer[:0]
				q.handler(batch)
				q.pool.Put(batch[:0])
			}
		default:
			if len(q.buffer) > 0 {
				batch := q.pool.Get().([]T)[:0]
				batch = append(batch, q.buffer...)
				q.handler(batch)
				q.pool.Put(batch[:0])
			}
			return
		}
	}
}

func (q *BufferQueue[T]) resetTimer() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.timer.Stop() {
		select {
		case <-q.timer.C:
		default:
		}
	}
	q.timer.Reset(q.maxWait)
}

// truncateMessage 消息截断
func truncateMessage(text string, maxMessageSize int) string {
	if utf8.RuneCountInString(text) <= maxMessageSize {
		return text
	}

	// 优先在换行符处截断
	if idx := strings.LastIndex(text[:maxMessageSize], "\n"); idx > 0 {
		return text[:idx] + "\n...(truncated)"
	}

	// 按字符截断
	runes := []rune(text)
	if len(runes) > maxMessageSize {
		runes = runes[:maxMessageSize-100]
	}
	return string(runes) + "...(truncated)"
}
