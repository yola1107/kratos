package alert

import (
	"context"
	"sync"

	"github.com/yola1107/kratos/v2/library/log/config"
	"github.com/yola1107/kratos/v2/log"
	"go.uber.org/zap/zapcore"
)

// Sender 发送接口，由具体实现提供
type Sender interface {
	Send(messages []string) error
	Close() error
}

// Alerter 报警器核心
type Alerter struct {
	sender    Sender
	processor *Processor
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// NewAlerter 创建报警器
func NewAlerter(config *config.Alert) *Alerter {
	if config == nil || !config.Enabled {
		return nil
	}
	tn, err := NewTelegramSender(config.Telegram)
	if err != nil {
		log.Infof("TelegramSender error: %+v", err)
		return nil
	}
	a := &Alerter{
		sender:    tn,
		processor: NewProcessor(config),
		stopChan:  make(chan struct{}),
	}

	a.wg.Add(1)
	go a.process()

	return a
}

// Write 实现 zapcore.WriteSyncer 接口
func (a *Alerter) Write(entry zapcore.Entry) error {
	return a.processor.Add(entry)
}

// Close 关闭报警器
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
