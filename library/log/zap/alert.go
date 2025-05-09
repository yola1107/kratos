package zap

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strings"
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

// Alerter 报警器核心
type Alerter struct {
	zapcore.LevelEnabler                 // 日志级别过滤器
	enc                  zapcore.Encoder // 日志编码器
	fields               []zapcore.Field // 附加字段

	conf     Alert            // 告警配置
	sender   Sender           // 消息发送器（如Telegram）
	msgChan  chan *tagMessage // 消息队列（带长度标记）
	stopChan chan struct{}    // 关闭信号
	wg       sync.WaitGroup   // 协程同步
	closed   int32            // 是否已关闭
	limiter  *rate.Limiter    // 限速器（防止消息轰炸）
}

type tagMessage struct {
	content string
	length  int
}

// NewAlerter 创建报警器
func NewAlerter(enabler zapcore.LevelEnabler, enc zapcore.Encoder, conf Alert) *Alerter {
	sender, err := NewTelegramSender(conf.Telegram)
	if err != nil {
		log.Warnf("Failed to create Alerter: %v", err)
		return nil
	}

	a := &Alerter{
		LevelEnabler: enabler,
		enc:          enc,
		conf:         conf,
		sender:       sender,
		msgChan:      make(chan *tagMessage, conf.QueueSize),
		stopChan:     make(chan struct{}),
		limiter:      rate.NewLimiter(rate.Every(conf.Limiter), 1),
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
	if atomic.LoadInt32(&a.closed) == 1 {
		return fmt.Errorf("alerter is closed")
	}

	// 空消息过滤
	if len(strings.TrimSpace(ent.Message)) == 0 {
		return nil
	}

	msg := truncateMessage(a.formatMessage(ent, append(a.fields, fields...)), maxTelegramMsgSize)

	select {
	case a.msgChan <- &tagMessage{content: msg, length: utf8.RuneCountInString(msg)}:
		return nil
	default:
		return fmt.Errorf("queue full (capacity=%d)", a.conf.QueueSize)
	}
}

// formatMessage 统一格式化日志消息
func (a *Alerter) formatMessage(ent zapcore.Entry, fields []zapcore.Field) string {
	var sb strings.Builder
	sb.Grow(256) // 预分配内存

	if a.conf.Prefix != "" {
		sb.WriteString("<" + a.conf.Prefix + ">   ")
	}

	sb.WriteString(time.Now().Format("2006-01-02 15:04:05.000"))
	sb.WriteString("    [")
	sb.WriteString(strings.ToUpper(ent.Level.String()))
	sb.WriteString("]    [")
	sb.WriteString(filepath.ToSlash(ent.Caller.FullPath()))
	sb.WriteString("]\n")
	sb.WriteString(ent.Message)

	if len(fields) > 0 {
		sb.WriteString("\n{")
		for i, f := range fields {
			sb.WriteString(fmt.Sprintf(`"%s":"%s"`, f.Key, f.String))
			if i < len(fields)-1 {
				sb.WriteString(", ")
			}
		}
		sb.WriteString("}")
	}

	return sb.String()
}

// Sync 同步日志
func (a *Alerter) Sync() error { return nil }

// Close 关闭
func (a *Alerter) Close() error {
	if !atomic.CompareAndSwapInt32(&a.closed, 0, 1) {
		return nil
	}
	close(a.stopChan) // 先关闭stopChan
	a.wg.Wait()
	return a.sender.Close() // 最后关闭sender
}

func (a *Alerter) process() {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("alerter panic. recover: %+v, %+v", r, debug.Stack())
		}
		a.wg.Done()
	}()

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
	return (msgLen+batchSize > maxTelegramMsgSize) || (batchCount >= a.conf.MaxBatchCnt)
}

func (a *Alerter) drainQueue(batch *[]string, batchSize *int) {
	for {
		select {
		case msg := <-a.msgChan:
			if a.needFlush(msg.length, *batchSize, len(*batch)) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := 0; i < a.conf.MaxRetries; i++ {
		if err := a.limiter.Wait(ctx); err != nil {
			log.Error("rate limit exceeded", zap.Error(err))
			break
		}

		if err := a.sender.Send(batch); err == nil {
			return
		}

		select {
		case <-time.After(time.Duration(i+1) * time.Second): // 指数退避
		case <-ctx.Done():
			return
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
