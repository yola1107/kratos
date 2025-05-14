package zap

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	maxTelegramMsgSize   = 4096 // Telegram消息最大长度 4k
)

const (
	defaultBatchCnt   = 10
	defaultQueueSize  = 4096
	defaultMaxRetries = 1
	defaultLimitRate  = time.Millisecond * 100
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

func WithPrefix(prefix string) Option {
	return func(c *Config) { c.prefix = prefix }
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
	prefix         string   // 前缀
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
		prefix:         "",
		telegramToken:  "",
		telegramChatID: "",
	}
	return cfg
}

type Logger struct {
	*zap.Logger
	level         zap.AtomicLevel
	closers       []io.Closer
	fieldPool     *fieldSlicePool
	sensitiveKeys map[string]struct{}
}

type fieldSlicePool struct {
	pool sync.Pool
}

func newFieldSlicePool() *fieldSlicePool {
	return &fieldSlicePool{
		pool: sync.Pool{
			New: func() any {
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
	var options = []zap.Option{
		zap.AddCaller(),
		zap.AddCallerSkip(2),
		zap.AddStacktrace(zapcore.PanicLevel)}

	// 日志级别
	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(cfg.level)); err != nil {
		level.SetLevel(zap.InfoLevel)
	}

	// 编码器配置
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeTime = customTimeEncoder
	encoderCfg.EncodeLevel = customColorLevelEncoder
	encoderCfg.EncodeCaller = zapcore.FullCallerEncoder
	encoderCfg.ConsoleSeparator = " "

	// Console
	cores = append(cores, zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), zapcore.Lock(os.Stderr), level))

	// File
	if !cfg.development && cfg.directory != "" && (cfg.filename != "" || cfg.errorFilename != "") {
		fileEncoderCfg := encoderCfg
		fileEncoderCfg.EncodeCaller = customCallerEncoder
		fileEncoderCfg.EncodeLevel = customLevelEncoder
		fileEnc := zapcore.NewConsoleEncoder(fileEncoderCfg)

		if cfg.directory != "" && cfg.filename != "" {
			lj := newLumberjack(cfg, cfg.filename)
			cores = append(cores, zapcore.NewCore(fileEnc, zapcore.AddSync(lj), level))
			closers = append(closers, lj)
		}
		if cfg.directory != "" && cfg.errorFilename != "" {
			lj := newLumberjack(cfg, cfg.errorFilename)
			cores = append(cores, zapcore.NewCore(fileEnc, zapcore.AddSync(lj), zapcore.ErrorLevel))
			closers = append(closers, lj)
		}
	}

	// Alerter
	if cfg.telegramToken != "" && cfg.telegramChatID != "" {
		alerter := NewAlerter(cfg.telegramToken, cfg.telegramChatID, cfg.prefix)
		options = append(options, zap.Hooks(alerter.Hook()))
		closers = append(closers, alerter)
	}

	// 脱敏 Core
	sensitiveKeys := make(map[string]struct{})
	for _, k := range cfg.sensitiveKeys {
		if k = strings.TrimSpace(k); k != "" {
			sensitiveKeys[strings.ToLower(k)] = struct{}{}
		}
	}

	logger := zap.New(zapcore.NewTee(cores...), options...)
	l := &Logger{Logger: logger, level: level, closers: closers, fieldPool: newFieldSlicePool(), sensitiveKeys: sensitiveKeys}
	log.Infof("zap logger initialized. cores=%d conf=%+v", len(cores), cfg)
	return l, nil
}

func newLumberjack(cfg *Config, filename string) *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename:   filepath.Join(cfg.directory, filename),
		MaxSize:    cfg.maxSizeMB,
		MaxAge:     cfg.maxAgeDays,
		MaxBackups: cfg.maxBackups,
		LocalTime:  cfg.localTime,
		Compress:   cfg.compress,
	}
}

func (l *Logger) Log(level log.Level, keyvals ...any) error {
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
				break
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
	return nil
}

// SetLevel 动态设置日志级别
func (l *Logger) SetLevel(level string) error {
	return l.level.UnmarshalText([]byte(level))
}

// With 方法创建子Logger，共享资源管理器
func (l *Logger) With(keys ...any) *Logger {
	fields := l.fieldPool.Get()
	defer l.fieldPool.Put(fields)

	for i := 0; i < len(keys); i += 2 {
		if i+1 >= len(keys) {
			fields = append(fields, zap.Any(fmt.Sprint(keys[i]), "(MISSING)"))
			continue
		}
		key, ok := keys[i].(string)
		if !ok {
			key = fmt.Sprintf("%v", keys[i])
		}
		fields = append(fields, zap.Any(key, keys[i+1]))
	}

	fields = l.filterSensitive(fields)

	newSensitiveKeys := make(map[string]struct{})
	for k := range l.sensitiveKeys {
		newSensitiveKeys[k] = struct{}{}
	}

	return &Logger{
		Logger:        l.Logger.With(fields...),
		level:         l.level,
		closers:       l.closers,
		fieldPool:     l.fieldPool,
		sensitiveKeys: newSensitiveKeys,
	}
}
