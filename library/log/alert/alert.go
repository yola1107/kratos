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
	maxTelegramMsgSize   = 4096 - 100 // Telegram消息最大长度 4k
	defaultRetryInterval = 1 * time.Second
	defaultMaxRetries    = 3
	defaultQueueSize     = 100
	defaultMaxInterval   = 5 * time.Second
	defaultMaxBatchCnt   = 10
	//sizeSafetyMargin     = 200 // 安全余量，防止刚好超过限制
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
	mu       sync.RWMutex
	isClosed bool

	sender Sender
}

// NewAlerter 创建报警器
func NewAlerter(enabler zapcore.LevelEnabler, enc zapcore.Encoder, conf *config.Alert) *Alerter {
	if conf == nil || !conf.Enabled {
		return nil
	}

	conf = validateConfig(conf)

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
		msgChan:      make(chan string, conf.QueueSize),
		stopChan:     make(chan struct{}),
	}

	a.wg.Add(1)
	go a.process()

	return a
}

// validateConfig 验证并设置默认配置
func validateConfig(conf *config.Alert) *config.Alert {
	if conf.QueueSize <= 0 {
		conf.QueueSize = defaultQueueSize
	}
	if conf.MaxInterval <= 0 {
		conf.MaxInterval = defaultMaxInterval
	}
	if conf.MaxBatchCnt <= 0 {
		conf.MaxBatchCnt = defaultMaxBatchCnt
	}
	if conf.MaxRetries <= 0 {
		conf.MaxRetries = defaultMaxRetries
	}
	return conf
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
		batchPool.Put(batch[:0])
	}()

	for {
		select {
		case <-a.stopChan:
			a.drainQueue(&batch, &batchSize)
			return

		case msg := <-a.msgChan:
			if a.shouldFlush(len(msg), batchSize, len(batch)) {
				a.sendWithRetry(batch)
				batch, batchSize = batch[:0], 0
			}
			batch = append(batch, msg)
			batchSize += len(msg)

		case <-ticker.C:
			if len(batch) > 0 {
				a.sendWithRetry(batch)
				batch, batchSize = batch[:0], 0
			}
		}
	}
}

func (a *Alerter) shouldFlush(msgLen, batchSize, batchCount int) bool {
	return (msgLen+batchSize > maxTelegramMsgSize) ||
		(batchCount >= a.conf.MaxBatchCnt)
}

func (a *Alerter) drainQueue(batch *[]string, batchSize *int) {
	for {
		select {
		case msg := <-a.msgChan:
			if a.shouldFlush(len(msg), *batchSize, len(*batch)) {
				a.sendWithRetry(*batch)
				*batch, *batchSize = (*batch)[:0], 0
			}
			*batch = append(*batch, msg)
			*batchSize += len(msg)
		default:
			return
		}
	}
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
			time.Sleep(time.Duration(i+1) * defaultRetryInterval)
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
