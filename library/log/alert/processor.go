package alert

import (
	"context"
	"fmt"
	"time"

	"github.com/yola1107/kratos/v2/library/log/config"
	"go.uber.org/zap/zapcore"
)

// Processor 处理管道
type Processor struct {
	config      *config.Alert
	batchChan   chan zapcore.Entry
	outputChan  chan []string
	rateLimiter *RateLimiter
}

// NewProcessor 创建处理器
func NewProcessor(config *config.Alert) *Processor {
	return &Processor{
		config:      config,
		batchChan:   make(chan zapcore.Entry, 1000),
		outputChan:  make(chan []string),
		rateLimiter: NewRateLimiter(config.RateLimit),
	}
}

// Add 添加日志条目
func (p *Processor) Add(entry zapcore.Entry) error {
	select {
	case p.batchChan <- entry:
	default:
		// 队列满时丢弃
	}
	return nil
}

// Start 启动处理管道
func (p *Processor) Start(ctx context.Context) <-chan []string {
	go p.batchWorker(ctx)
	go p.rateLimitWorker(ctx)
	return p.outputChan
}

// Stop 停止处理器
func (p *Processor) Stop() {
	close(p.batchChan)
}

func (p *Processor) batchWorker(ctx context.Context) {
	var batch []zapcore.Entry

	ticker := time.NewTicker(p.config.Batch.MaxInterval)
	defer ticker.Stop()

	for {
		select {
		case entry, ok := <-p.batchChan:
			if !ok {
				if len(batch) > 0 {
					p.sendToRateLimiter(formatMessages(batch))
				}
				return
			}

			batch = append(batch, entry)
			if len(batch) >= p.config.Batch.MaxSize {
				p.sendToRateLimiter(formatMessages(batch))
				batch = nil
			}

		case <-ticker.C:
			if len(batch) > 0 {
				p.sendToRateLimiter(formatMessages(batch))
				batch = nil
			}

		case <-ctx.Done():
			return
		}
	}
}

func (p *Processor) sendToRateLimiter(messages []string) {
	select {
	case p.outputChan <- messages:
	default:
		// 限流器处理不过来时丢弃
	}
}

func (p *Processor) rateLimitWorker(ctx context.Context) {
	for messages := range p.outputChan {
		if p.rateLimiter.Allow() {
			p.outputChan <- messages
		}
	}
	close(p.outputChan)
}

func formatMessages(entries []zapcore.Entry) []string {
	var messages []string
	for _, entry := range entries {
		msg := fmt.Sprintf("[%s] %s", entry.Level.CapitalString(), entry.Message)
		if len(entry.Caller.TrimmedPath()) > 0 {
			msg += fmt.Sprintf("\nLocation: %s", entry.Caller.TrimmedPath())
		}
		messages = append(messages, msg)
	}
	return messages
}
