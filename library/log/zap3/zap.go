package zap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/yola1107/kratos/v2/log"
)

var _ log.Logger = (*Logger)(nil)

const (
	defaultFieldCapacity = 32
	maxPoolCapacity      = 1024
	sensitiveMask        = "***"
	timeFormat           = "2006/01/02 15:04:05.000"

	maxTelegramMsgSize = 4096 // Telegram消息最大长度 4k
)

const (
	defaultBatchCnt   = 10
	defaultQueueSize  = 1000
	defaultMaxRetries = 1
)

type Option func(*Config)

func WithDevelopment() Option {
	return func(c *Config) { c.development = true }
}

type Config struct {
	development    bool     // "dev" 或 "prod"
	Level          string   // 日志级别: debug/info/warn/error
	Directory      string   // 日志文件目录
	FileName       string   // 普通日志文件名
	ErrorFileName  string   // 错误日志文件名
	MaxSizeMB      int      // 单文件最大 MB
	MaxBackups     int      // 最大备份数
	MaxAgeDays     int      // 最大保留天数
	Compress       bool     // 是否压缩历史日志
	LocalTime      bool     // 使用本地时间戳
	SensitiveKeys  []string // 敏感字段名或前缀，不区分大小写
	TelegramToken  string   // Telegram Bot Token
	TelegramChatID string   // Telegram Chat ID
}

func defaultConfig() *Config {
	cfg := &Config{
		development:    true,
		Level:          "debug",
		Directory:      "",
		FileName:       "",
		ErrorFileName:  "",
		MaxSizeMB:      200,
		MaxBackups:     10,
		MaxAgeDays:     7,
		Compress:       true,
		LocalTime:      true,
		SensitiveKeys:  []string{},
		TelegramToken:  "token",
		TelegramChatID: "chatId",
	}
	return cfg
}

type Logger struct {
	*zap.Logger
	level         zap.AtomicLevel
	closers       []io.Closer
	fieldPool     *fieldSlicePool
	alerter       *Alerter
	sensitiveKeys map[string]struct{}
}

type fieldSlicePool struct {
	pool sync.Pool
}

func newFieldSlicePool() *fieldSlicePool {
	return &fieldSlicePool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]zap.Field, 0, defaultFieldCapacity)
			},
		},
	}
}

func (p *fieldSlicePool) Get() []zap.Field {
	return p.pool.Get().([]zap.Field)[:0]
}

func (p *fieldSlicePool) Put(fields []zap.Field) {
	if cap(fields) <= maxPoolCapacity {
		p.pool.Put(fields[:0])
	}
}

func NewLogger(opts ...Option) (*Logger, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	var cores []zapcore.Core
	var closers []io.Closer

	// 日志级别
	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level.SetLevel(zap.InfoLevel)
	}

	// 编码器配置
	encoderCfg := zap.NewProductionEncoderConfig()
	if cfg.development {
		encoderCfg.EncodeCaller = zapcore.FullCallerEncoder
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout(timeFormat)
		encoderCfg.ConsoleSeparator = " "
	}
	consoleEnc := zapcore.NewConsoleEncoder(encoderCfg)
	jsonEnc := zapcore.NewJSONEncoder(encoderCfg)

	// Console
	cores = append(cores, zapcore.NewCore(consoleEnc, zapcore.AddSync(os.Stdout), level))

	// File
	if cfg.Directory != "" && cfg.FileName != "" {
		lj := &lumberjack.Logger{
			Filename:   filepath.Join(cfg.Directory, cfg.FileName),
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays,
			Compress:   cfg.Compress,
			LocalTime:  cfg.LocalTime,
		}
		cores = append(cores, zapcore.NewCore(jsonEnc, zapcore.AddSync(lj), level))
		closers = append(closers, lj)
	}
	if cfg.Directory != "" && cfg.ErrorFileName != "" {
		lj := &lumberjack.Logger{
			Filename:   filepath.Join(cfg.Directory, cfg.ErrorFileName),
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays,
			Compress:   cfg.Compress,
			LocalTime:  cfg.LocalTime,
		}
		cores = append(cores, zapcore.NewCore(jsonEnc, zapcore.AddSync(lj), zapcore.ErrorLevel))
		closers = append(closers, lj)
	}

	core := zapcore.NewTee(cores...)

	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(2), zap.AddStacktrace(zapcore.PanicLevel))
	l := &Logger{Logger: logger, level: level, closers: closers, fieldPool: newFieldSlicePool(), sensitiveKeys: make(map[string]struct{})}

	// 脱敏 Core
	for _, k := range cfg.SensitiveKeys {
		l.sensitiveKeys[strings.ToLower(k)] = struct{}{}
	}

	// Alerter
	if cfg.TelegramToken != "" && cfg.TelegramChatID != "" {
		alerter := NewAlerter(cfg.TelegramToken, cfg.TelegramChatID)
		l.alerter = alerter
		l.Logger = logger.WithOptions(zap.Hooks(alerter.Hook()))
	}

	return l, nil
}

func (l *Logger) Log(level log.Level, keyvals ...interface{}) error {
	if len(keyvals) == 0 {
		return nil
	}

	var msg string
	fields := l.fieldPool.Get()
	defer l.fieldPool.Put(fields)

	// 预分配字段容量
	expectedFields := len(keyvals)/2 + 1
	if cap(fields) < expectedFields {
		fields = make([]zap.Field, 0, expectedFields)
	}

	for i := 0; i < len(keyvals); i += 2 {
		if i+1 >= len(keyvals) {
			fields = append(fields, zap.Any(fmt.Sprint(keyvals[i]), "(MISSING)"))
			continue
		}

		key, ok := keyvals[i].(string)
		if !ok {
			key = fmt.Sprintf("%v", keyvals[i])
		}

		if key == log.DefaultMessageKey {
			msg, _ = keyvals[i+1].(string)
			continue
		}

		fields = append(fields, zap.Any(key, keyvals[i+1]))
	}

	fields = l.filterSensitive(fields)

	switch level {
	case log.LevelDebug:
		l.Debug(msg, fields...)
	case log.LevelInfo:
		l.Info(msg, fields...)
	case log.LevelWarn:
		l.Warn(msg, fields...)
	case log.LevelError:
		l.Error(msg, fields...)
	case log.LevelFatal:
		l.Fatal(msg, fields...)
	}
	return nil
}

// 敏感信息过滤
func (l *Logger) filterSensitive(fields []zap.Field) []zap.Field {
	for i, field := range fields {
		if _, ok := l.sensitiveKeys[strings.ToLower(field.Key)]; ok {
			fields[i] = zap.String(field.Key, sensitiveMask)
		}
	}
	return fields
}

func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// Close 关闭资源
func (l *Logger) Close() error {
	_ = l.Sync()
	for _, c := range l.closers {
		_ = c.Close()
	}
	if l.alerter != nil {
		return l.alerter.Close()
	}
	log.Info("zap Logger closed successfully")
	return nil
}

// SetLevel 动态设置日志级别
func (l *Logger) SetLevel(level string) error {
	var lv zapcore.Level
	if err := lv.UnmarshalText([]byte(level)); err != nil {
		return err
	}
	l.level.SetLevel(lv)
	return nil
}

/*
	Alerter
*/

type Alerter struct {
	token  string
	chatID string
	queue  chan *tagMessage
	wg     sync.WaitGroup
	quit   chan struct{}
	closed atomic.Bool // 是否已关闭
	client *http.Client
}

type tagMessage struct {
	content string
	length  int
}

func NewAlerter(token, chatID string) *Alerter {
	a := &Alerter{
		token:  token,
		chatID: chatID,
		queue:  make(chan *tagMessage, defaultQueueSize),
		quit:   make(chan struct{}),
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
			payload := map[string]interface{}{
				"level":  e.Level.String(),
				"msg":    e.Message,
				"time":   e.Time.Format(timeFormat),
				"caller": e.Caller.TrimmedPath(),
			}
			if b, err := json.MarshalIndent(payload, "", "  "); err == nil {
				select {
				case a.queue <- &tagMessage{content: string(b), length: len(b)}:
				default:
					log.Warnf("queue full (capacity=%d)", defaultQueueSize)
					fmt.Printf("queue full (capacity=%d)", defaultQueueSize)
				}
			}
		}
		return nil
	}
}

func (a *Alerter) sender() {
	defer a.wg.Done()

	var (
		batchPool = sync.Pool{New: func() interface{} { return make([]string, 0, defaultBatchCnt) }}
		batch     = batchPool.Get().([]string)
		batchSize int
	)
	defer batchPool.Put(batch[:0])

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.quit:
			a.drainQueue(&batch, &batchSize)
			a.sendWithRetry(batch)
			return

		case msg := <-a.queue:
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

func (a *Alerter) drainQueue(batch *[]string, batchSize *int) {
	timeout := time.After(100 * time.Millisecond)
loop:
	for {
		select {
		case msg, ok := <-a.queue:
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

	for attempt := 1; attempt <= defaultMaxRetries; attempt++ {
		if err := a.send(batch); err == nil {
			return
		}
		select {
		case <-time.After(time.Duration(attempt) * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

func (a *Alerter) send(batch []string) error {
	content := ""
	for _, msg := range batch {
		content += msg
		content += "\n\n" // 用两个换行分隔多条消息
	}
	content += "\n\n---------\n\n"

	//fmt.Printf("=========>%+v send %d \n", time.Now().Format("2006-01-02 15:04:05.000"), len(messages))
	//fmt.Printf("=========>%+v send %d content: \n%+v", time.Now().Format("2006-01-02 15:04:05.000"), len(batch), content)
	//return nil

	_, err := a.client.PostForm(
		"https://api.telegram.org/bot"+a.token+"/sendMessage",
		url.Values{
			"chat_id": {a.chatID},
			"text":    {content},
		},
	)
	if err != nil {
		fmt.Printf("%+v %v\n", time.Now().Format("2006-01-02 15:04:05.000"), err)
	}
	return err
}

func (a *Alerter) Close() error {
	if !a.closed.CompareAndSwap(false, true) {
		return nil
	}

	close(a.quit)
	a.wg.Wait()
	remaining := len(a.queue)
	close(a.queue)
	a.client.CloseIdleConnections()

	log.Infof("alerter closed. remaining: %d", remaining)
	return nil
}
