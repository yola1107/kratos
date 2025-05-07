package alert

import (
	"context"
	"sync"

	"go.uber.org/zap/zapcore"

	"github.com/yola1107/kratos/v2/library/log/config"
	"github.com/yola1107/kratos/v2/log"
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

	sender    Sender
	processor *Processor
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// NewAlerter 创建报警器
func NewAlerter(enabler zapcore.LevelEnabler, enc zapcore.Encoder, config *config.Alert) *Alerter {
	if config == nil || !config.Enabled {
		return nil
	}
	tn, err := NewTelegramSender(config.Telegram)
	if err != nil {
		log.Infof("TelegramSender error: %+v", err)
		return nil
	}
	a := &Alerter{
		LevelEnabler: enabler,
		enc:          enc,

		sender:    tn,
		processor: NewProcessor(config),
		stopChan:  make(chan struct{}),
	}

	a.wg.Add(1)
	go a.process()

	return a
}

// With 添加字段
func (a *Alerter) With(fields []zapcore.Field) zapcore.Core {
	// 即使 fields 为空也返回新实例
	clone := &Alerter{
		LevelEnabler: a.LevelEnabler,
		enc:          a.enc,

		sender:    a.sender,
		processor: a.processor,
		stopChan:  a.stopChan,
	}

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

	return a.processor.Add(truncateMessage(entryBuf.String()))
}

// Sync 同步日志
func (a *Alerter) Sync() error { return nil }

// Close 关闭
func (a *Alerter) Close() error {
	close(a.stopChan)
	a.wg.Wait()
	return a.sender.Close()
}

func (a *Alerter) process() {
	defer a.wg.Done()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动处理器
	outputChan := a.processor.Start(ctx)

	for {
		select {
		case messages, ok := <-outputChan:
			if !ok {
				return
			}
			_ = a.sender.Send(messages)
		case <-a.stopChan:
			a.processor.Stop()
			return
		}
	}
}
