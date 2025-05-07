package alert

import (
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
	maxTelegramMsgSize = 4096 - 100 // Telegram消息最大长度 4k
	retryInterval      = 1 * time.Second
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
	msgChan  chan string
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
	isClosed bool

	sender Sender
}

// NewAlerter 创建报警器
func NewAlerter(enabler zapcore.LevelEnabler, enc zapcore.Encoder, config *config.Alert) *Alerter {
	if config == nil || !config.Enabled {
		return nil
	}

	// 设置默认值
	if config.QueueSize <= 0 {
		config.QueueSize = 100
	}
	if config.MaxInterval <= 0 {
		config.MaxInterval = 5 * time.Second
	}
	if config.MaxBatchCnt <= 0 {
		config.MaxBatchCnt = 10
	}

	sender, err := NewTelegramSender(config.Telegram)
	if err != nil {
		log.Errorf("Failed to create Telegram sender: %v", err)
		return nil
	}

	a := &Alerter{
		LevelEnabler: enabler,
		enc:          enc,
		conf:         config,
		sender:       sender,
		msgChan:      make(chan string, config.QueueSize),
		stopChan:     make(chan struct{}),
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

	clone := &Alerter{
		LevelEnabler: a.LevelEnabler,
		enc:          a.enc.Clone(),
		conf:         a.conf,
		sender:       a.sender,
		msgChan:      a.msgChan,
		stopChan:     a.stopChan,
		wg:           a.wg,
	}

	clone.fields = make([]zapcore.Field, len(a.fields))
	copy(clone.fields, a.fields)
	clone.fields = append(clone.fields, fields...)

	return clone
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
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isClosed {
		return fmt.Errorf("alerter is closed")
	}

	entryBuf, err := a.enc.EncodeEntry(ent, append(a.fields, fields...))
	if err != nil {
		return fmt.Errorf("failed to encode entry: %w", err)
	}

	msg := truncateMessage(entryBuf.String(), maxTelegramMsgSize)
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
	close(a.stopChan)
	a.mu.Unlock()

	a.wg.Wait()

	if err := a.sender.Close(); err != nil {
		return fmt.Errorf("failed to close sender: %w", err)
	}
	return nil
}

func (a *Alerter) enqueueMessage(msg string) error {
	select {
	case a.msgChan <- msg:
		return nil
	default:
		return fmt.Errorf("queue full (capacity=%d)", a.conf.QueueSize)
	}
}

func (a *Alerter) process() {
	defer a.wg.Done()

	var (
		batch     = make([]string, 0, a.conf.MaxBatchCnt)
		batchSize int
		ticker    = time.NewTicker(a.conf.MaxInterval)
	)
	defer ticker.Stop()

	maxSendRetries := a.conf.MaxRetries // 最大重试次数
	sendWithRetry := func(msgs []string) {
		for i := 0; i < maxSendRetries; i++ {
			if err := a.sender.Send(msgs); err == nil {
				return
			}
			if i < maxSendRetries-1 {
				time.Sleep(retryInterval)
			}
		}
		log.Errorf("Failed to send batch after %d retries", maxSendRetries)
	}

	flushBatch := func() {
		if len(batch) > 0 {
			sendWithRetry(batch)
			batch = batch[:0] // 重用slice
			batchSize = 0
		}
	}

	drainQueue := func() {
		for {
			select {
			case msg := <-a.msgChan:
				if len(msg)+batchSize > maxTelegramMsgSize || len(batch) >= a.conf.MaxBatchCnt {
					flushBatch()
				}
				batch = append(batch, msg)
				batchSize += len(msg)
			default:
				flushBatch()
				return
			}
		}
	}

	for {
		select {
		case <-a.stopChan:
			drainQueue()
			return

		case msg := <-a.msgChan:
			if len(msg)+batchSize > maxTelegramMsgSize || len(batch) >= a.conf.MaxBatchCnt {
				flushBatch()
			}
			batch = append(batch, msg)
			batchSize += len(msg)

		case <-ticker.C:
			flushBatch()
		}
	}
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
