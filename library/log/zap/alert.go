package zap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"

	"github.com/yola1107/kratos/v2/library/log/zap/conf"
	"github.com/yola1107/kratos/v2/log"
)

const (
	defaultMaxMsgSize    = 4096
	defaultQueueSize     = 2048
	defaultMaxRetries    = 1
	defaultBatchSize     = 10
	defaultFlushInterval = 3 * time.Second
	defaultRateLimit     = 100 * time.Millisecond
)

type Sender interface {
	SendMessage(string) error
	Close() error
}

type Alert struct {
	c      *conf.Alerter
	sender Sender
	zapcore.LevelEnabler
	enc    zapcore.Encoder
	fields []zapcore.Field
	mu     sync.Mutex

	// 消息队列相关字段
	quit    chan struct{}
	queue   chan string
	closed  atomic.Bool
	limiter *rate.Limiter
	wg      sync.WaitGroup
}

func NewAlert(c *conf.Alerter, sender Sender) *Alert {
	alert := &Alert{
		c:            c,
		sender:       sender,
		LevelEnabler: zap.ErrorLevel,
		enc:          zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		quit:         make(chan struct{}),
		queue:        make(chan string, defaultQueueSize),
		limiter:      rate.NewLimiter(rate.Every(defaultRateLimit), 1),
	}
	alert.wg.Add(1)
	go alert.run()
	log.Debugf("Alert initialized. enable:%v prefix:%q format:%q", c.Enabled, c.Prefix, c.Format)
	return alert
}

func (a *Alert) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if a.Enabled(ent.Level) {
		return ce.AddCore(ent, a)
	}
	return ce
}

func (a *Alert) With(fields []zapcore.Field) zapcore.Core {
	clone := *a
	clone.fields = append(a.fields[:len(a.fields):len(a.fields)], fields...)
	return &clone
}

func (a *Alert) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	if !a.c.Enabled || a.closed.Load() || ent.Level < zap.ErrorLevel {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// 合并基础字段和日志字段
	allFields := append(a.fields, fields...)
	msg, err := a.formatMessage(ent, allFields)
	if err != nil {
		return err
	}

	select {
	case a.queue <- msg:
	default:
		// 队列满时丢弃消息
		// log.Warn("alerter queue full, dropping message", zap.Int("queue_cap", cap(a.queue)))
	}
	return nil
}

func (a *Alert) formatMessage(e zapcore.Entry, fields []zapcore.Field) (string, error) {
	switch a.c.Format {
	case "json":
		return a.formatJSON(e, fields)
	default:
		return a.formatHTML(e, fields)
	}
}

func (a *Alert) formatJSON(e zapcore.Entry, fields []zapcore.Field) (string, error) {
	data := map[string]any{
		"level":  e.Level.String(),
		"msg":    e.Message,
		"time":   e.Time.Format(timeFormat),
		"caller": e.Caller.FullPath(),
		"prefix": a.c.Prefix,
	}

	// 使用 Zap 的编码器提取字段值
	enc := zapcore.NewMapObjectEncoder()
	for _, field := range fields {
		field.AddTo(enc)
	}

	// 添加字段到数据中
	for k, v := range enc.Fields {
		data[k] = v
	}

	b, err := json.MarshalIndent(data, "", "  ")
	return string(b), err
}

func (a *Alert) formatHTML(e zapcore.Entry, fields []zapcore.Field) (string, error) {
	var sb strings.Builder
	sb.WriteString("<b>[" + e.Level.String() + "]</b> ")
	sb.WriteString("<code>" + e.Time.Format(timeFormat) + "</code>\n")

	if a.c.Prefix != "" {
		sb.WriteString("<b>APP:</b> " + htmlEscape(a.c.Prefix) + "\n")
	}

	// 使用 Zap 的编码器提取字段值
	enc := zapcore.NewMapObjectEncoder()
	for _, field := range fields {
		field.AddTo(enc)
	}

	// 添加字段部分
	if len(enc.Fields) > 0 {
		sb.WriteString("<b>Fields:</b>\n")
		for k, v := range enc.Fields {
			value := fmt.Sprintf("%v", v)
			sb.WriteString("  • <b>" + htmlEscape(k) + ":</b> " + htmlEscape(value) + "\n")
		}
	}

	sb.WriteString("<b>Msg:</b> " + htmlEscape(e.Message) + "\n")
	sb.WriteString("<b>Caller:</b> <code>" + htmlEscape(e.Caller.FullPath()) + "</code>")

	return sb.String(), nil
}

func htmlEscape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	).Replace(s)
}

func (a *Alert) Sync() error { return nil }

func (a *Alert) Close() error {
	if !a.closed.CompareAndSwap(false, true) {
		return nil
	}

	start := time.Now()
	close(a.quit)
	a.wg.Wait()

	err := a.sender.Close()
	log.Infof("alerter stopped. remaining=%d duration=%v", len(a.queue), time.Since(start))
	return err
}

func (a *Alert) run() {
	defer recoverPanic()
	defer a.wg.Done()

	var (
		batch     []string
		batchSize int
	)

	ticker := time.NewTicker(defaultFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.quit:
			// 丢弃剩余消息
			return
		case msg := <-a.queue:
			if a.needFlush(len(msg), batchSize, len(batch)) {
				a.flush(batch)
				batch, batchSize = nil, 0
			}
			batch = append(batch, msg)
			batchSize += len(msg)
		case <-ticker.C:
			a.flush(batch)
			batch, batchSize = nil, 0
		}
	}
}

func (a *Alert) needFlush(msgSize, batchSize, count int) bool {
	return count >= defaultBatchSize || msgSize+batchSize >= defaultMaxMsgSize
}

func (a *Alert) flush(batch []string) {
	if len(batch) == 0 {
		return
	}

	content := strings.Join(batch, "\n\n")
	if len(content) > defaultMaxMsgSize {
		content = content[:defaultMaxMsgSize-50] + "\n[...]"
	}
	ctx := context.Background()
	for i := 0; i < defaultMaxRetries; i++ {
		if err := a.limiter.Wait(ctx); err != nil {
			log.Warn("alerter rate limit", zap.Error(err))
			return
		}
		// fmt.Printf("=========>%+v send %d content: \n%+v", time.Now().Format(timeFormat), len(batch), content)
		if err := a.sender.SendMessage(content); err == nil {
			return
		}
		if i < defaultMaxRetries-1 {
			time.Sleep(time.Second << i)
		}
	}
}

func recoverPanic() {
	if r := recover(); r != nil {
		log.Error("alerter panic recovered",
			zap.Any("reason", r),
			zap.String("stack", string(debug.Stack())))
	}
}

// ----------------------------------------------------------------------------------
// telegram

type Telegram struct {
	c      *conf.Telegram
	client *http.Client
}

func NewTelegram(c *conf.Telegram) *Telegram {
	return &Telegram{
		c: c,
		client: &http.Client{
			Timeout: 1000 * time.Millisecond,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: true,
			},
		},
	}
}

func (tg *Telegram) SendMessage(content string) error {
	if tg.c.Token == "" || tg.c.ChatID == "" {
		return nil
	}
	resp, err := tg.client.PostForm(
		"https://api.telegram.org/bot"+tg.c.Token+"/sendMessage",
		url.Values{
			"chat_id":    {tg.c.ChatID},
			"text":       {content},
			"parse_mode": {"HTML"},
		},
	)
	if err != nil {
		log.Warn("telegram send error", zap.Error(err))
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (tg *Telegram) Close() error {
	tg.client.CloseIdleConnections()
	log.Info("telegram stopped.")
	return nil
}
