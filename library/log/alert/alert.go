package alert

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"

	"github.com/yola1107/kratos/v2/library/log/config"
	"github.com/yola1107/kratos/v2/log"
)

const (
	maxTelegramMsgSize = 4096 - 100 // Telegram消息最大长度 4k
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

	conf     *config.Alert
	sender   Sender
	msgChan  chan tagMessage
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.RWMutex
	isClosed bool
	limiter  *rate.Limiter // 限速器
}

type tagMessage struct {
	content string
	length  int
}

// NewAlerter 创建报警器
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
		msgChan:      make(chan tagMessage, conf.QueueSize),
		stopChan:     make(chan struct{}),
		limiter:      rate.NewLimiter(rate.Every(300*time.Millisecond), 1),
	}

	a.wg.Add(1)
	go a.process()

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

// Write 写入日志
func (a *Alerter) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.isClosed {
		return fmt.Errorf("alerter is closed")
	}

	entryBuf, err := a.enc.EncodeEntry(ent, append(a.fields, fields...))
	if err != nil {
		return fmt.Errorf("failed to encode entry: %w", err)
	}

	// 空消息过滤
	if len(strings.TrimSpace(entryBuf.String())) == 0 {
		return nil
	}

	msg := truncateMessage(a.conf.Prefix+entryBuf.String(), maxTelegramMsgSize)
	return a.enqueueMessage(msg)
}

// Sync 同步日志
func (a *Alerter) Sync() error { return nil }

// Close 关闭
func (a *Alerter) Close() error {
	a.mu.Lock()
	if a.isClosed {
		a.mu.Unlock()
		return nil
	}
	a.isClosed = true
	close(a.stopChan) // 先关闭stopChan
	a.mu.Unlock()

	a.wg.Wait() // 等待处理协程退出

	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sender.Close() // 最后关闭sender
}

func (a *Alerter) enqueueMessage(msg string) error {
	qm := tagMessage{
		content: msg,
		length:  utf8.RuneCountInString(msg),
	}
	select {
	case a.msgChan <- qm:
		return nil
	default:
		return fmt.Errorf("queue full (capacity=%d)", a.conf.QueueSize)
	}
}

func (a *Alerter) process() {
	defer func() {
		if r := recover(); r != nil {
			log.Error("alerter process panic", zap.Any("recover", r))
		}
		a.wg.Done()
	}()

	var (
		batchPool = sync.Pool{
			New: func() interface{} {
				return make([]string, 0, a.conf.MaxBatchCnt)
			},
		}
		batch     = batchPool.Get().([]string)
		batchSize int
		ticker    = time.NewTicker(a.conf.MaxInterval)
	)
	defer func() {
		ticker.Stop()
		if len(batch) > 0 {
			a.sendWithRetry(batch)
		}
		batchPool.Put(batch[:0:cap(batch)])
	}()

	for {
		select {
		case <-a.stopChan:
			a.drainQueue(&batch, &batchSize)
			return

		case msg := <-a.msgChan:
			if a.shouldFlush(msg.length, batchSize, len(batch)) {
				a.sendWithRetry(batch)
				batch, batchSize = batch[:0], 0
			}
			batch = append(batch, msg.content)
			batchSize += msg.length

		case <-ticker.C:
			if len(batch) > 0 {
				a.sendWithRetry(batch)
				batch, batchSize = batch[:0], 0
			}
		}
	}
}

func (a *Alerter) shouldFlush(msgLen, batchSize, batchCount int) bool {
	return (msgLen+batchSize > maxTelegramMsgSize) || (batchCount >= a.conf.MaxBatchCnt)
}

func (a *Alerter) drainQueue(batch *[]string, batchSize *int) {
	for {
		select {
		case msg := <-a.msgChan:
			if a.shouldFlush(msg.length, *batchSize, len(*batch)) {
				a.sendWithRetry(*batch)
				*batch, *batchSize = (*batch)[:0], 0
			}
			*batch = append(*batch, msg.content)
			*batchSize += msg.length
		default:
			return
		}
	}
}

func (a *Alerter) sendWithRetry(batch []string) {
	if len(batch) == 0 {
		return
	}
	// 限速控制：阻塞直到可以发送一批
	if err := a.limiter.Wait(context.Background()); err != nil {
		log.Errorf("Rate limiter wait error: %v", err)
		return
	}
	for i := 0; i < a.conf.MaxRetries; i++ {
		if err := a.sender.Send(batch); err == nil {
			return
		}
		if i < a.conf.MaxRetries-1 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}
}

// truncateMessage 消息截断
func truncateMessage(text string, maxSize int) string {
	if utf8.RuneCountInString(text) <= maxSize {
		return text
	}

	// 优先在换行符处截断
	if idx := strings.LastIndex(text[:maxSize], "\n"); idx > 0 {
		return text[:idx] + "\n...(truncated)"
	}

	// 按字符截断
	runes := []rune(text)
	if len(runes) > maxSize {
		runes = runes[:maxSize-100]
	}
	return string(runes) + "...(truncated)"
}
