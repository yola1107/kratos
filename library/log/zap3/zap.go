package zap

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
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
	maxTelegramMsgSize   = 4096 // Telegram消息最大长度 4k
)

const (
	defaultBatchCnt   = 10
	defaultQueueSize  = 2048
	defaultMaxRetries = 1
)

type Option func(*Config)

func WithProduction() Option {
	return func(c *Config) { c.development = false }
}

func WithLevel(level string) Option {
	return func(c *Config) { c.level = level }
}

func WithDirectory(dir string) Option {
	return func(c *Config) { c.directory = dir }
}

func WithFilename(filename string) Option {
	return func(c *Config) { c.filename = filename }
}

func WithErrorFilename(filename string) Option {
	return func(c *Config) { c.errorFilename = filename }
}

func WithMaxSizeMB(maxSizeMB int) Option {
	return func(c *Config) { c.maxSizeMB = maxSizeMB }
}

func WithMaxAgeDays(MaxAge int) Option {
	return func(c *Config) { c.maxAgeDays = MaxAge }
}

func WithMaxBackups(maxBackups int) Option {
	return func(c *Config) { c.maxBackups = maxBackups }
}

func WithCompress(compress bool) Option {
	return func(c *Config) { c.compress = compress }
}

func WithLocalTime(localTime bool) Option {
	return func(c *Config) { c.localTime = localTime }
}

func WithSensitiveKeys(SensitiveKeys []string) Option {
	return func(c *Config) { c.sensitiveKeys = SensitiveKeys }
}
func WithToken(token string) Option {
	return func(c *Config) { c.telegramToken = token }
}

func WithChatID(chatID string) Option {
	return func(c *Config) { c.telegramChatID = chatID }
}

type Config struct {
	development    bool     // "dev" 或 "prod"
	level          string   // 日志级别: debug/info/warn/error
	directory      string   // 日志文件目录
	filename       string   // 普通日志文件名
	errorFilename  string   // 错误日志文件名
	maxSizeMB      int      // 单文件最大 MB
	maxBackups     int      // 最大备份数
	maxAgeDays     int      // 最大保留天数
	compress       bool     // 是否压缩历史日志
	localTime      bool     // 使用本地时间戳
	sensitiveKeys  []string // 敏感字段名或前缀，不区分大小写
	telegramToken  string   // Telegram Bot Token
	telegramChatID string   // Telegram Chat ID
}

func defaultConfig() *Config {
	cfg := &Config{
		development:    true,
		level:          "debug",
		directory:      "",
		filename:       "",
		errorFilename:  "",
		maxSizeMB:      200,
		maxBackups:     10,
		maxAgeDays:     7,
		compress:       true,
		localTime:      true,
		sensitiveKeys:  []string{},
		telegramToken:  "token",
		telegramChatID: "chatId",
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
	if err := level.UnmarshalText([]byte(cfg.level)); err != nil {
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

	// Console zapcore.AddSync(os.Stdout)
	cores = append(cores, zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), zapcore.Lock(os.Stderr), level))

	// File
	if !cfg.development && cfg.directory != "" && (cfg.filename != "" || cfg.errorFilename != "") {
		if err := os.MkdirAll(cfg.directory, 0755); err != nil {
			return nil, fmt.Errorf("create log directory failed: %v", err)
		}

		fileEncoderCfg := encoderCfg
		fileEncoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder
		jsonEnc := zapcore.NewJSONEncoder(fileEncoderCfg)

		if cfg.directory != "" && cfg.filename != "" {
			lj := &lumberjack.Logger{
				Filename:   filepath.Join(cfg.directory, cfg.filename),
				MaxSize:    cfg.maxSizeMB,
				MaxBackups: cfg.maxBackups,
				MaxAge:     cfg.maxAgeDays,
				Compress:   cfg.compress,
				LocalTime:  cfg.localTime,
			}
			cores = append(cores, zapcore.NewCore(jsonEnc, zapcore.AddSync(lj), level))
			closers = append(closers, lj)
		}
		if cfg.directory != "" && cfg.errorFilename != "" {
			lj := &lumberjack.Logger{
				Filename:   filepath.Join(cfg.directory, cfg.errorFilename),
				MaxSize:    cfg.maxSizeMB,
				MaxBackups: cfg.maxBackups,
				MaxAge:     cfg.maxAgeDays,
				Compress:   cfg.compress,
				LocalTime:  cfg.localTime,
			}
			cores = append(cores, zapcore.NewCore(jsonEnc, zapcore.AddSync(lj), zapcore.ErrorLevel))
			closers = append(closers, lj)
		}
	}

	core := zapcore.NewTee(cores...)

	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(2), zap.AddStacktrace(zapcore.PanicLevel))
	l := &Logger{Logger: logger, level: level, closers: closers, fieldPool: newFieldSlicePool(), sensitiveKeys: make(map[string]struct{})}

	// 脱敏 Core
	for _, k := range cfg.sensitiveKeys {
		l.sensitiveKeys[strings.ToLower(k)] = struct{}{}
	}

	// Alerter
	if cfg.telegramToken != "" && cfg.telegramChatID != "" {
		alerter := NewAlerter(cfg.telegramToken, cfg.telegramChatID)
		l.alerter = alerter
		l.Logger = logger.WithOptions(zap.Hooks(alerter.Hook()))
	}

	log.Infof("zap logger initialized. cores=%d conf=%+v", len(cores), cfg)
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
		keyLower := strings.ToLower(field.Key)
		for sensitiveKey := range l.sensitiveKeys {
			if strings.HasPrefix(keyLower, sensitiveKey) {
				fields[i] = zap.String(field.Key, sensitiveMask)
				break // 匹配任意前缀即脱敏
			}
		}
	}
	return fields
}

func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// Close 关闭资源
func (l *Logger) Close() error {
	defer log.Info("zap Logger closed successfully")
	_ = l.Sync()
	for _, c := range l.closers {
		_ = c.Close()
	}
	if l.alerter != nil {
		return l.alerter.Close()
	}
	return nil
}

// SetLevel 动态设置日志级别
func (l *Logger) SetLevel(level string) error {
	return l.level.UnmarshalText([]byte(level))
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
			if msg, err := toJSONMsg(e); err == nil {
				select {
				case a.queue <- &tagMessage{content: msg, length: len(msg)}:
				default:
					log.Warnf("queue full (capacity=%d)", defaultQueueSize)
				}
			}
		}

		return nil
	}
}

func toJSONMsg(e zapcore.Entry) (string, error) {
	payload := map[string]interface{}{
		"level":  e.Level.String(),
		"msg":    e.Message,
		"time":   e.Time.Format(timeFormat),
		"caller": e.Caller.TrimmedPath(),
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	return string(b), err
}

func (a *Alerter) sender() {
	defer a.recoverPanic()
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
	//if len(batch) == 0 || a.closed.Load() {
	if len(batch) == 0 {
		return
	}
	for attempt := 1; attempt <= defaultMaxRetries; attempt++ {
		if err := a.send(batch); err == nil {
			return
		}
		// 不是最后一次才 sleep
		if attempt < defaultMaxRetries {
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}
	}
}

var sendCount int64

func (a *Alerter) send(batch []string) error {
	var sb strings.Builder
	for _, msg := range batch {
		sb.WriteString(msg + "\n\n")
	}
	sb.WriteString("\n---------\n\n")
	content := sb.String()

	sendCount += int64(len(batch))

	//fmt.Printf("=========>%+v send %d \n", time.Now().Format(timeFormat), len(batch))
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
		fmt.Printf("%+v %v\n", time.Now().Format(timeFormat), err)
	}
	return err
}

func (a *Alerter) Close() error {
	if !a.closed.CompareAndSwap(false, true) {
		return nil
	}

	stopTime := time.Now()

	fmt.Printf("=== close at: %s SendedCnt=%d queue=%d\n", time.Now().Format(timeFormat), sendCount, len(a.queue))
	close(a.quit)
	a.wg.Wait()
	close(a.queue)
	a.client.CloseIdleConnections()

	log.Infof("alerter closed. sendCnt=%d remaining:%d usetime=%+v", sendCount, len(a.queue), time.Now().Sub(stopTime))
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
