package zap

import (
	"context"
	"encoding/json"
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

	"github.com/yola1107/kratos/v2/log"
)

/*
	Alerter
*/

type Alerter struct {
	token   string
	chatID  string
	prefix  string
	queue   chan *tagMessage
	wg      sync.WaitGroup
	quit    chan struct{}
	limiter *rate.Limiter // 限速器（防止消息轰炸）
	closed  atomic.Bool   // 是否已关闭
	client  *http.Client
}

type tagMessage struct {
	content string
	length  int
}

func NewAlerter(token, chatID string, prefix string) *Alerter {
	a := &Alerter{
		token:   token,
		chatID:  chatID,
		prefix:  prefix,
		queue:   make(chan *tagMessage, defaultQueueSize),
		quit:    make(chan struct{}),
		limiter: rate.NewLimiter(rate.Every(defaultLimitRate), 1), // 1秒10条的告警消息 每条最多defaultBatchCnt个单消息
		client: &http.Client{
			Timeout: 1 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    10 * time.Second,
				DisableCompression: true,
			}},
	}
	a.wg.Add(1)
	go a.sender()
	return a
}

func (a *Alerter) Hook() func(zapcore.Entry) error {
	return func(e zapcore.Entry) error {
		if a.closed.Load() {
			return nil
		}
		if e.Level >= zapcore.ErrorLevel {
			if msg, err := toJSONMsg(e, a.prefix); err == nil {
				select {
				case <-a.quit: // 优先检查退出信号
					return nil
				case a.queue <- &tagMessage{content: msg, length: len(msg)}:
				default:
					log.Warnf("queue full (capacity=%d)", cap(a.queue))
				}
			}
		}

		return nil
	}
}

func toJSONMsg(e zapcore.Entry, prefix string) (string, error) {
	payload := map[string]any{
		"level":  e.Level.String(),
		"msg":    e.Message,
		"time":   e.Time.Format(timeFormat),
		"caller": e.Caller.FullPath(),
		"prefix": prefix,
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	return string(b), err
}

func (a *Alerter) sender() {
	defer a.recoverPanic()
	defer a.wg.Done()

	var (
		batchPool = sync.Pool{New: func() any { return make([]string, 0, defaultBatchCnt) }}
		batch     = batchPool.Get().([]string)
		batchSize int
	)
	defer batchPool.Put(batch[:0])

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.quit:
			// 丢弃剩余报警消息
			return

		case msg, ok := <-a.queue:
			if !ok {
				return //stop时的空消息
			}
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
	return msgLen+batchSize > maxTelegramMsgSize || batchCount >= defaultBatchCnt
}

func (a *Alerter) sendWithRetry(batch []string) {
	if len(batch) == 0 {
		return
	}
	ctx := context.Background()
	for attempt := 1; attempt <= defaultMaxRetries; attempt++ {
		if err := a.limiter.Wait(ctx); err != nil {
			log.Warn("rate limit exceeded", zap.Error(err))
			return
		}
		if err := a.send(batch); err == nil {
			return
		}
		// 不是最后一次才 sleep
		if attempt < defaultMaxRetries {
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}
	}
}

func (a *Alerter) send(batch []string) error {
	var sb strings.Builder
	for _, msg := range batch {
		sb.WriteString(msg + "\n\n")
	}
	sb.WriteString("\n---------\n\n")
	content := sb.String()

	//fmt.Printf("=========>%+v send %d content: \n%+v", time.Now().Format(timeFormat), len(batch), content)
	//return nil

	// 截断保护
	if len(content) > maxTelegramMsgSize {
		content = content[:maxTelegramMsgSize-50] + "\n\n[...truncated]"
	}
	_, err := a.client.PostForm(
		"https://api.telegram.org/bot"+a.token+"/sendMessage",
		url.Values{
			"chat_id": {a.chatID},
			"text":    {content},
		},
	)
	if err != nil {
		log.Warnf("%+v", err)
	}
	return err
}

func (a *Alerter) Close() error {
	if !a.closed.CompareAndSwap(false, true) {
		return nil
	}
	start := time.Now()
	defer func() {
		log.Infof("alerter closed. remaining:%d usetime=%+v", len(a.queue), time.Now().Sub(start))
	}()

	close(a.quit)
	a.wg.Wait()
	close(a.queue)
	a.client.CloseIdleConnections()
	return nil
}

func (a *Alerter) recoverPanic() {
	if r := recover(); r != nil {
		log.Error("alerter panic recovered",
			zap.Any("reason", r),
			zap.String("stack", string(debug.Stack())),
		)
	}
}
