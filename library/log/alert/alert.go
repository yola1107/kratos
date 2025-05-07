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
)

// Sender 发送接口，由具体实现提供
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

	sender Sender
}

// NewAlerter 创建报警器
func NewAlerter(enabler zapcore.LevelEnabler, enc zapcore.Encoder, config *config.Alert) *Alerter {
	if config == nil || !config.Enabled {
		return nil
	}
	sender, err := NewTelegramSender(config.Telegram)
	if err != nil {
		log.Infof("TelegramSender error: %+v", err)
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

	// 即使 fields 为空也返回新实例
	clone := &Alerter{
		LevelEnabler: a.LevelEnabler,
		enc:          a.enc,
		conf:         a.conf,
		sender:       a.sender,
		msgChan:      a.msgChan,
		stopChan:     a.stopChan,
	}

	//clone := *a

	// 完全拷贝所有字段
	clone.fields = make([]zapcore.Field, len(a.fields), len(a.fields)+len(fields))
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
	entryBuf, err := a.enc.EncodeEntry(ent, append(a.fields, fields...))
	if err != nil {
		return err
	}

	msg := truncateMessage(entryBuf.String(), maxTelegramMsgSize)
	return a.enqueueMessage(msg)
}

// Sync 同步日志
func (a *Alerter) Sync() error { return nil }

// Close 关闭
func (a *Alerter) Close() error {
	close(a.stopChan)
	a.wg.Wait()
	return a.sender.Close()
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
		ticker  = time.NewTicker(a.conf.MaxInterval)
		ticker2 = time.NewTicker(time.Millisecond * 200)
	)
	defer ticker.Stop()
	defer ticker2.Stop()

	for {
		select {
		case <-a.stopChan:
			// 处理剩余消息
			for len(a.msgChan) > 0 {
				a.sendBatch()
			}
		case <-ticker2.C:
			a.sendBatch()
		case <-ticker.C:
			a.sendBatch()
		}
	}
}

func (a *Alerter) sendBatch() {
	if len(a.msgChan) <= 0 {
		return
	}

	batchSize := 0
	batch := make([]string, 0, a.conf.MaxBatchCnt)

	for i := 0; i < a.conf.MaxBatchCnt; i++ {
		msg := <-a.msgChan
		if len(msg)+batchSize > maxTelegramMsgSize || len(batch) >= a.conf.MaxBatchCnt {
			_ = a.sender.Send(batch)
			batch = batch[:0] // 重用slice
			batchSize = 0
		}
		batch = append(batch, msg)
		batchSize += len(msg)
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
