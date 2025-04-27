package zap

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/yola1107/kratos/v2/log"
	"go.uber.org/zap/zapcore"
)

const (
	maxMessageSize    = 4096 - 100             // Telegram消息最大长度 4k
	minRateLimit      = 500 * time.Millisecond // 最小速率限制
	defaultQueueSize  = 100                    // 默认队列容量
	defaultBatchCnt   = 5                      // 默认批量消息
	defaultMaxRetries = 1                      // 默认重试次数
)

// Notifier 通知器接口
type Notifier interface {
	Notify(text string) error
	Close() error
}

// AlertCore 报警核心
type AlertCore struct {
	zapcore.LevelEnabler
	enc      zapcore.Encoder
	fields   []zapcore.Field
	pool     sync.Pool
	notifier Notifier

	writeMu sync.Mutex // 全局写入锁 保证消息顺序
}

// NewAlertCore 创建报警核心
func NewAlertCore(enabler zapcore.LevelEnabler, enc zapcore.Encoder, cfg *TelegramConfig) *AlertCore {
	n := NewTelegramNotifier(cfg)
	if n == nil {
		return nil
	}
	return &AlertCore{
		LevelEnabler: enabler,
		enc:          enc,
		pool: sync.Pool{
			New: func() interface{} { return new(bytes.Buffer) },
		},
		notifier: n,
	}
}

// With 添加字段
func (c *AlertCore) With(fields []zapcore.Field) zapcore.Core {
	// 即使 fields 为空也返回新实例
	clone := &AlertCore{
		LevelEnabler: c.LevelEnabler,
		enc:          c.enc,
		pool: sync.Pool{
			New: c.pool.New, // 复用原New函数
		},
		notifier: c.notifier,
	}

	// 完全拷贝所有字段
	clone.fields = make([]zapcore.Field, len(c.fields), len(c.fields)+len(fields))
	copy(clone.fields, c.fields)
	clone.fields = append(clone.fields, fields...)

	return clone
}

// Check 检查日志级别
func (c *AlertCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

// Write 写入日志
func (c *AlertCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	buf := c.pool.Get().(*bytes.Buffer)
	buf.Reset()
	defer c.pool.Put(buf)

	entryBuf, err := c.enc.EncodeEntry(ent, append(c.fields, fields...))
	if err != nil {
		return err
	}
	return c.notifier.Notify(truncateMessage(entryBuf.String()))
}

// Sync 同步日志
func (c *AlertCore) Sync() error { return nil }

// Close 关闭
func (c *AlertCore) Close() error { return c.notifier.Close() }

/*
 * Telegram 通知器实现
 */

// ringBuffer 环形缓冲区
type ringBuffer struct {
	mu       sync.Mutex
	msgs     []string
	head     int
	tail     int
	count    int
	capacity int
	maxTake  int
}

// newRingBuffer 创建环形缓冲区
func newRingBuffer(capacity int, maxTake int) *ringBuffer {
	if capacity <= 0 {
		capacity = defaultQueueSize
	}
	if maxTake <= 0 {
		maxTake = defaultBatchCnt
	}
	return &ringBuffer{
		msgs:     make([]string, capacity),
		capacity: capacity,
		maxTake:  maxTake,
	}
}

// Add 添加消息到队列
func (r *ringBuffer) push(msg string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count >= r.capacity {
		return false
	}

	r.msgs[r.tail] = msg
	r.tail = (r.tail + 1) % r.capacity
	r.count++
	return true
}

// popN 从队列取出消息
func (r *ringBuffer) popN() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count == 0 {
		return nil
	}

	var msgs []string
	totalSize := 0
	taken := 0

	for r.count > 0 && taken < r.maxTake {
		curr := r.msgs[r.head]
		currSize := len(curr)

		if totalSize+currSize >= maxMessageSize {
			//第一条超出
			if taken == 0 {
				msgs = append(msgs, r.popHead())
				taken++
			}
			break
		}

		msgs = append(msgs, r.popHead())
		totalSize += currSize
		taken++
	}

	return msgs
}

func (r *ringBuffer) popHead() string {
	msg := r.msgs[r.head]
	r.msgs[r.head] = ""
	r.head = (r.head + 1) % r.capacity
	r.count--
	return msg
}

// ReturnToHead 将消息返回到队列头部
func (r *ringBuffer) ReturnToHead(msgs []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	free := r.capacity - r.count
	n := len(msgs)

	// 丢弃旧的（前面的）多余消息
	if n > free {
		msgs = msgs[n-free:]
		n = len(msgs)
	}

	// 回退 head 的位置
	newHead := r.head
	for i := n - 1; i >= 0; i-- {
		newHead = (newHead - 1 + r.capacity) % r.capacity
		r.msgs[newHead] = msgs[i]
	}
	r.head = newHead
	r.count += n
}

// Count 当前队列中的消息数量
func (r *ringBuffer) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

func (r *ringBuffer) Free() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.capacity - r.count
}

/*
 * Telegram 通知器实现
 */

// telegramNotifier 核心实现
type telegramNotifier struct {
	config        *TelegramConfig
	client        *http.Client
	ring          *ringBuffer
	closeOnce     sync.Once
	closeChan     chan struct{}
	forceSendChan chan struct{}
	wg            sync.WaitGroup
}

// NewTelegramNotifier 创建实例
func NewTelegramNotifier(cfg *TelegramConfig) Notifier {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	if err := cfg.validate(); err != nil {
		log.Warnf("%+v", err)
		return nil
	}
	tn := &telegramNotifier{
		config:        cfg,
		client:        &http.Client{Timeout: 5 * time.Second},
		ring:          newRingBuffer(cfg.QueueSize, cfg.MaxBatchCnt),
		closeChan:     make(chan struct{}),
		forceSendChan: make(chan struct{}),
	}

	tn.wg.Add(1)
	go tn.processor() // 启动处理协程
	return tn
}

// Notify 发送通知（非阻塞）
func (tn *telegramNotifier) Notify(text string) error {
	if !tn.ring.push(text) {
		select {
		case tn.forceSendChan <- struct{}{}:
			return nil
		default:
			return fmt.Errorf("queue full (capacity=%d)", tn.config.QueueSize)
		}
	}
	return nil
}

// processMessages 主处理循环
func (tn *telegramNotifier) processor() {
	defer tn.wg.Done()

	ticker := time.NewTicker(tn.config.RateLimit)
	defer ticker.Stop()

	for {
		select {
		case <-tn.closeChan:
			// 关闭时处理剩余消息
			for tn.ring.Count() > 0 {
				tn.sendBatch(tn.ring.popN())
			}
			return

		case <-tn.forceSendChan:
			if msgs := tn.ring.popN(); len(msgs) > 0 {
				tn.sendBatch(msgs)
			}

		case <-ticker.C:
			if msgs := tn.ring.popN(); len(msgs) > 0 {
				tn.sendBatch(msgs)
			}
		}
	}
}

// sendBatch 批量发送
func (tn *telegramNotifier) sendBatch(messages []string) {
	if len(messages) == 0 {
		return
	}

	// 合并消息
	var sb strings.Builder
	for _, msg := range messages {
		sb.WriteString(tn.config.Prefix)
		sb.WriteString(msg)
		//sb.WriteByte('\n')
	}

	// 带重试的发送
	for i := 0; i < tn.config.MaxRetries; i++ {
		if err := tn.send(sb.String()); err == nil {
			return
		}
		if i < tn.config.MaxRetries-1 {
			time.Sleep(time.Second * time.Duration(i+1))
		}
	}
}

// send 实际发送请求
func (tn *telegramNotifier) send(content string) error {
	//go func() {
	//fmt.Printf("=========>%+v send content: \n%+v", time.Now().Format("2006-01-02 15:04:05.000"), content)
	//return nil
	_, err := tn.client.PostForm(
		"https://api.telegram.org/bot"+tn.config.Token+"/sendMessage",
		url.Values{
			"chat_id": {tn.config.ChatID},
			"text":    {content},
		},
	)
	if err != nil {
		fmt.Printf(" %v\n", err)
	}
	return err
	//}()

}

// Close 安全关闭
func (tn *telegramNotifier) Close() error {
	tn.closeOnce.Do(func() {
		close(tn.closeChan)
		tn.wg.Wait()
		tn.client.CloseIdleConnections()
		log.Infof("telegram closed")
	})
	return nil
}

// truncateMessage 消息截断
func truncateMessage(text string) string {
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
