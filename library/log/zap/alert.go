package zap

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"

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

type SafeSender struct {
	mu     sync.Mutex
	sender Sender
}

func (s *SafeSender) Send(messages []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sender.Send(messages)
}

func (s *SafeSender) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sender.Close()
}

type Alerter struct {
	zapcore.LevelEnabler                 // 日志级别过滤器
	enc                  zapcore.Encoder // 日志编码器
	fields               []zapcore.Field // 附加字段

	conf     Alert            // 告警配置
	sender   SafeSender       // 消息发送器（如Telegram）
	msgChan  chan *tagMessage // 消息队列（带长度标记）
	stopChan chan struct{}    // 关闭信号
	wg       sync.WaitGroup   // 协程同步
	closed   atomic.Bool      // 是否已关闭
	limiter  *rate.Limiter    // 限速器（防止消息轰炸）
}

type tagMessage struct {
	content string
	length  int
}

func NewAlerter(enabler zapcore.LevelEnabler, enc zapcore.Encoder, conf Alert, sender Sender) *Alerter {
	a := &Alerter{
		LevelEnabler: enabler,
		enc:          enc,
		conf:         conf,
		sender:       SafeSender{sender: sender},
		msgChan:      make(chan *tagMessage, conf.QueueSize),
		stopChan:     make(chan struct{}),
		limiter:      rate.NewLimiter(rate.Every(conf.LimitPolicy.Limit), conf.LimitPolicy.Burst),
	}

	a.wg.Add(1)
	go a.process()

	return a
}

func (a *Alerter) With(fields []zapcore.Field) zapcore.Core {
	if len(fields) == 0 {
		return a
	}

	clone := *a
	clone.enc = a.enc.Clone()
	clone.fields = append(append([]zapcore.Field{}, a.fields...), fields...)

	return &clone
}

func (a *Alerter) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if a.Enabled(ent.Level) {
		return ce.AddCore(ent, a)
	}
	return ce
}

func (a *Alerter) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	if a.closed.Load() {
		return fmt.Errorf("alerter closed")
	}

	fullFields := append(append(a.fields, fields...), zap.String("prefix", a.conf.Prefix))
	entryBuf, err := a.enc.EncodeEntry(ent, fullFields)
	if err != nil || entryBuf == nil {
		return fmt.Errorf("encode error: %w", err)
	}
	msg := truncateMessage(formatJSONString(entryBuf.String()), maxTelegramMsgSize)

	select {
	case a.msgChan <- &tagMessage{content: msg, length: utf8.RuneCountInString(msg)}:
		return nil
	default:
		return fmt.Errorf("queue full (capacity=%d)", a.conf.QueueSize)
	}
}

func formatJSONString(input string) string {
	var obj interface{}
	if err := json.Unmarshal([]byte(input), &obj); err != nil {
		return input // 不是JSON则原样返回
	}

	prettyJSON, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return input
	}
	return string(prettyJSON)
}

func (a *Alerter) Sync() error { return nil }

func (a *Alerter) Close() error {
	if !a.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(a.stopChan)
	a.wg.Wait()
	close(a.msgChan)
	defer log.Infof("alerter closed. remaining messages: %d", len(a.msgChan))
	return a.sender.Close()
}

func (a *Alerter) process() {
	defer a.recoverPanic()
	defer a.wg.Done()

	var (
		batchPool = sync.Pool{New: func() interface{} { return make([]string, 0, a.conf.MaxBatchCnt) }}
		batch     = batchPool.Get().([]string)
		batchSize int
	)
	defer batchPool.Put(batch[:0])

	ticker := time.NewTicker(a.conf.MaxInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopChan:
			a.drainQueue(&batch, &batchSize)
			a.sendWithRetry(batch)
			return

		case msg := <-a.msgChan:
			if a.needFlush(msg.length, batchSize, len(batch)) {
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

func (a *Alerter) needFlush(msgLen, batchSize, batchCount int) bool {
	return msgLen+batchSize > maxTelegramMsgSize || batchCount >= a.conf.MaxBatchCnt
}

func (a *Alerter) drainQueue(batch *[]string, batchSize *int) {
	timeout := time.After(100 * time.Millisecond)
loop:
	for {
		select {
		case msg, ok := <-a.msgChan:
			if !ok {
				log.Info("msgChan closed, exit drainQueue")
				break loop
			}
			if a.needFlush(msg.length, *batchSize, len(*batch)) {
				a.sendWithRetry(*batch)
				*batch, *batchSize = (*batch)[:0], 0
			}
			*batch = append(*batch, msg.content)
			*batchSize += msg.length
		case <-timeout:
			break loop
		default:
			break loop
		}
	}
	// 发送最终批次
	if len(*batch) > 0 {
		a.sendWithRetry(*batch)
	}
}

func (a *Alerter) sendWithRetry(batch []string) {
	if len(batch) == 0 || a.closed.Load() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for attempt := 1; attempt <= a.conf.RetryPolicy.MaxRetries; attempt++ {
		if err := a.limiter.Wait(ctx); err != nil {
			log.Warn("rate limit exceeded", zap.Error(err))
			return
		}

		if err := a.sender.Send(batch); err == nil {
			return
		}

		select {
		case <-time.After(a.calculateBackoff(attempt)):
		case <-ctx.Done():
			return
		}
	}
}

func (a *Alerter) calculateBackoff(attempt int) time.Duration {
	base := time.Duration(attempt) * a.conf.RetryPolicy.Backoff
	if base < a.conf.RetryPolicy.MinInterval {
		base = a.conf.RetryPolicy.MinInterval
	}
	return base
}

func (a *Alerter) recoverPanic() {
	if r := recover(); r != nil {
		log.Error("alerter panic recovered",
			zap.Any("reason", r),
			zap.String("stack", string(debug.Stack())),
		)
	}
}
func truncateMessage(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}

	// 优先在结构边界截断
	truncatePoints := []rune{'\n', '}', ']', ';'}
	runes := []rune(s)
	for i := max - 1; i > max/2; i-- {
		for _, p := range truncatePoints {
			if runes[i] == p {
				return string(runes[:i+1]) + "\n...[truncated]"
			}
		}
	}

	// 保留重要信息头尾
	head := string(runes[:max/2])
	tail := string(runes[len(runes)-max/2:])
	return head + "\n...[truncated]\n" + tail
}
