package alert

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/yola1107/kratos/v2/library/log/config"
	"github.com/yola1107/kratos/v2/log"
	"go.uber.org/zap/zapcore"
)

const (
	maxTelegramMsgSize = 4096 // Telegram消息最大长度 4k
)

// Sender 发送接口
type Sender interface {
	Send(messages []string) error
	Close() error
}

type Alerter struct {
	zapcore.LevelEnabler
	enc    zapcore.Encoder
	fields []zapcore.Field
	conf   *config.Alert

	sender Sender
	queue  *BufferQueue[string]
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
	entryBuf, err := a.enc.EncodeEntry(ent, append(a.fields, fields...))
	if err != nil {
		return fmt.Errorf("failed to encode entry: %w", err)
	}

	msg := truncateMessage(entryBuf.String(), maxTelegramMsgSize)
	return a.queue.Push(msg)
}

func (a *Alerter) Sync() error { return nil }

func (a *Alerter) Close() error {
	a.queue.Close()         // 确保所有批次已处理完
	return a.sender.Close() // 安全关闭 sender
}

func (a *Alerter) sendWithRetry(msgs []string) {
	if len(msgs) == 0 {
		return
	}

	for i := 0; i < a.conf.MaxRetries; i++ {
		if err := a.sender.Send(msgs); err == nil {
			return
		}
		if i < a.conf.MaxRetries-1 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}
}

// ======================== BufferQueue ========================

type BufferQueue[T any] struct {
	ch       chan T
	stopChan chan struct{}
	wg       sync.WaitGroup // 主协程

	batchSize int
	maxWait   time.Duration
	handler   func([]T)

	mu     sync.Mutex
	buffer []T
	timer  *time.Timer

	sendWG sync.WaitGroup // 用于等待所有 handler 完成
}

func NewBufferQueue[T any](conf *config.Alert, handler func([]T)) *BufferQueue[T] {
	q := &BufferQueue[T]{
		ch:        make(chan T, conf.QueueSize),
		stopChan:  make(chan struct{}),
		batchSize: conf.MaxBatchCnt,
		maxWait:   conf.MaxInterval,
		handler:   handler,
		timer:     time.NewTimer(conf.MaxInterval),
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
	q.wg.Wait()     // 等待主 process 协程退出
	q.sendWG.Wait() // 等待所有发送任务完成
}

func (q *BufferQueue[T]) process() {
	defer q.wg.Done()
	defer q.timer.Stop()

	for {
		select {
		case item := <-q.ch:
			q.mu.Lock()
			q.buffer = append(q.buffer, item)
			shouldFlush := len(q.buffer) >= q.batchSize
			q.mu.Unlock()

			if shouldFlush {
				q.flush()
			} else {
				q.resetTimer()
			}

		case <-q.timer.C:
			q.flush()

		case <-q.stopChan:
			q.flush() // 尽量 flush 所有剩余
			return    // 退出循环
		}
	}
}

func (q *BufferQueue[T]) flush() {
	q.mu.Lock()
	if len(q.buffer) == 0 {
		q.mu.Unlock()
		return
	}

	batch := make([]T, len(q.buffer))
	copy(batch, q.buffer)
	q.buffer = q.buffer[:0]
	q.mu.Unlock()

	q.sendWG.Add(1)
	go func(batch []T) {
		defer func() {
			_ = recover()
			q.sendWG.Done()
		}()
		q.handler(batch)
	}(batch)

	q.resetTimer()
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

// ======================== 辅助函数 ========================

func truncateMessage(text string, maxMessageSize int) string {
	if utf8.RuneCountInString(text) <= maxMessageSize {
		return text
	}

	if idx := strings.LastIndex(text[:maxMessageSize], "\n"); idx > 0 {
		return text[:idx] + "\n...(truncated)"
	}

	runes := []rune(text)
	if len(runes) > maxMessageSize {
		runes = runes[:maxMessageSize-100]
	}
	return string(runes) + "...(truncated)"
}
