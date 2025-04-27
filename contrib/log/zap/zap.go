package zap

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/yola1107/kratos/v2/log"
)

var _ log.Logger = (*Logger)(nil)

const (
	defaultFieldCapacity = 16 // Default capacity for field slices
	sensitiveMask        = "******"
)

type Mode string

const (
	Development Mode = "dev"
	Production  Mode = "prod"
)

type Config struct {
	Mode          Mode            `yaml:"mode" json:"mode"`
	Level         string          `yaml:"level" json:"level"`
	Directory     string          `yaml:"directory" json:"directory"`
	Filename      string          `yaml:"filename" json:"filename"`
	ErrorFilename string          `yaml:"error_filename" json:"error_filename"`
	MaxSize       int             `yaml:"max_size" json:"max_size"`
	MaxAge        int             `yaml:"max_age" json:"max_age"`
	MaxBackups    int             `yaml:"max_backups" json:"max_backups"`
	FlushInterval time.Duration   `yaml:"flush_interval" json:"flush_interval"`
	Compress      bool            `yaml:"compress" json:"compress"`
	QueueSize     int             `yaml:"queue_size" json:"queue_size"`
	PoolSize      int             `yaml:"pool_size" json:"pool_size"`
	LocalTime     bool            `yaml:"local_time" json:"local_time"`
	SensitiveKeys []string        `yaml:"sensitive_keys" json:"sensitive_keys"`
	Telegram      *TelegramConfig `yaml:"telegram" json:"telegram"`

	HookFunc func(zapcore.Entry, []zapcore.Field) `yaml:"-" json:"-"`
}

// TelegramConfig 配置参数
type TelegramConfig struct {
	Enabled     bool          `yaml:"enabled"`       // 是否启用
	Token       string        `yaml:"token"`         // Bot Token
	ChatID      string        `yaml:"chat_id"`       // 聊天ID
	Threshold   zapcore.Level `yaml:"threshold"`     // 日志级别
	QueueSize   int           `yaml:"queue_size"`    // 队列大小
	RateLimit   time.Duration `yaml:"rate_limit"`    // 发送间隔
	MaxBatchCnt int           `yaml:"max_batch_cnt"` // 最大批量数
	MaxRetries  int           `yaml:"max_retries"`   // 最大重试
	Prefix      string        `yaml:"prefix"`        // 消息前缀
}

func (tc *TelegramConfig) validate() error {
	if tc == nil || !tc.Enabled {
		return nil
	}
	if tc.Token == "" || tc.ChatID == "" {
		return errors.New("telegram token and chat_id are required when enabled")
	}

	if tc.QueueSize <= 0 {
		tc.QueueSize = defaultQueueSize
	}
	if tc.RateLimit < minRateLimit {
		tc.RateLimit = minRateLimit
	}
	if tc.MaxBatchCnt <= 0 {
		tc.MaxBatchCnt = defaultBatchCnt
	}
	if tc.MaxRetries <= 0 {
		tc.MaxRetries = defaultMaxRetries
	}
	return nil
}

func DefaultConfig() *Config {
	return &Config{
		Mode:          Development,
		Level:         "debug",
		Directory:     "./logs",
		Filename:      "app.log",
		ErrorFilename: "error.log",
		MaxSize:       50,
		MaxAge:        7,
		MaxBackups:    3,
		FlushInterval: 1 * time.Second,
		Compress:      false,
		LocalTime:     true,
		QueueSize:     512,
		PoolSize:      128,
		SensitiveKeys: []string{"password", "token", "secret"},
		Telegram: &TelegramConfig{
			Enabled:     false,
			Token:       os.Getenv("TG_TOKEN"),
			ChatID:      os.Getenv("TG_CHAT_ID"),
			Threshold:   zapcore.ErrorLevel,
			QueueSize:   100,
			RateLimit:   3 * time.Second,
			MaxBatchCnt: 5,
			MaxRetries:  1,
		},
	}
}

type Logger struct {
	*zap.Logger
	config    *Config
	encoder   zapcore.EncoderConfig
	level     zap.AtomicLevel
	resources []io.Closer
	fieldPool *sync.Pool
	closeOnce sync.Once
}

// New creates a new Logger instance. Panics on initialization errors.
func New(cfg *Config) *Logger {
	logger, err := newLogger(cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %v", err))
	}
	return logger
}

// NewWithError creates a new Logger instance and returns any initialization error.
func NewWithError(cfg *Config) (*Logger, error) {
	return newLogger(cfg)
}

func newLogger(cfg *Config) (*Logger, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	if err := cfg.Telegram.validate(); err != nil {
		return nil, fmt.Errorf("invalid telegram config: %w", err)
	}

	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level.SetLevel(zap.InfoLevel)
	}

	encoderConfig := getEncoderConfig(cfg.Mode)

	cores, resources, err := createCore(cfg, level, encoderConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create log cores: %w", err)
	}

	return &Logger{
		config:    cfg,
		encoder:   encoderConfig,
		level:     level,
		resources: resources,
		Logger: zap.New(
			zapcore.NewTee(cores...),
			zap.AddCaller(),
			zap.AddCallerSkip(2),
			zap.AddStacktrace(zap.PanicLevel),
		),
		fieldPool: &sync.Pool{
			New: func() interface{} {
				return make([]zap.Field, 0, defaultFieldCapacity)
			},
		},
	}, nil
}

func createCore(cfg *Config, level zap.AtomicLevel, encoderConfig zapcore.EncoderConfig) ([]zapcore.Core, []io.Closer, error) {
	var (
		cores     []zapcore.Core
		resources []io.Closer
	)

	// Production mode core
	if cfg.Mode == Production {
		if err := os.MkdirAll(cfg.Directory, 0755); err != nil {
			return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		infoWriter := &lumberjack.Logger{
			Filename:   filepath.Join(cfg.Directory, cfg.Filename),
			MaxSize:    cfg.MaxSize,
			MaxAge:     cfg.MaxAge,
			MaxBackups: cfg.MaxBackups,
			Compress:   cfg.Compress,
			LocalTime:  cfg.LocalTime,
		}
		errorWriter := &lumberjack.Logger{
			Filename:   filepath.Join(cfg.Directory, cfg.ErrorFilename),
			MaxSize:    cfg.MaxSize,
			MaxAge:     cfg.MaxAge,
			MaxBackups: cfg.MaxBackups,
			Compress:   cfg.Compress,
			LocalTime:  cfg.LocalTime,
		}
		resources = append(resources, infoWriter, errorWriter)

		encoder := zapcore.NewJSONEncoder(encoderConfig)
		cores = append(cores,
			zapcore.NewCore(encoder, zapcore.AddSync(infoWriter), level),
			zapcore.NewCore(encoder, zapcore.AddSync(errorWriter), zap.ErrorLevel),
		)
	}

	// Telegram core
	if cfg.Telegram != nil && cfg.Telegram.Enabled {
		alertCore := NewAlertCore(
			zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				return lvl >= cfg.Telegram.Threshold
			}),
			zapcore.NewConsoleEncoder(encoderConfig),
			cfg.Telegram,
		)
		if alertCore != nil {
			cores = append(cores, alertCore)
			resources = append(resources, alertCore)
		}
	}

	// Always add console core
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	cores = append(cores, zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stderr), level))

	return cores, resources, nil
}

func getEncoderConfig(mod Mode) zapcore.EncoderConfig {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:          "ts",
		LevelKey:         "level",
		NameKey:          "logger",
		CallerKey:        "caller",
		FunctionKey:      zapcore.OmitKey,
		MessageKey:       "msg",
		StacktraceKey:    "stack",
		LineEnding:       zapcore.DefaultLineEnding,
		EncodeLevel:      zapcore.LowercaseLevelEncoder,
		EncodeTime:       zapcore.ISO8601TimeEncoder,
		EncodeDuration:   zapcore.StringDurationEncoder,
		EncodeCaller:     zapcore.ShortCallerEncoder,
		ConsoleSeparator: "| ",
	}
	if mod == Development {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderConfig.EncodeCaller = zapcore.FullCallerEncoder
		encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
		encoderConfig.ConsoleSeparator = " "
	}
	return encoderConfig
}

func (l *Logger) Log(level log.Level, keyvals ...interface{}) error {
	if len(keyvals) == 0 {
		return nil
	}

	var msg string
	fields := l.fieldPool.Get().([]zap.Field)
	defer func() {
		fields = fields[:0] // Reset slice
		l.fieldPool.Put(fields)
	}()

	for i := 0; i < len(keyvals); i += 2 {
		if i+1 >= len(keyvals) {
			fields = append(fields, zap.Any(fmt.Sprint(keyvals[i]), "(MISSING)"))
			continue
		}

		key, ok := keyvals[i].(string)
		if !ok {
			key = fmt.Sprintf("%v", keyvals[i])
		}

		if key == l.encoder.MessageKey {
			msg, _ = keyvals[i+1].(string)
			continue
		}

		fields = append(fields, zap.Any(key, keyvals[i+1]))
	}

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

func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

func (l *Logger) Close() error {
	var errs []error

	l.closeOnce.Do(func() {
		if err := l.Sync(); err != nil {
			errs = append(errs, fmt.Errorf("sync error: %w", err))
		}

		for _, res := range l.resources {
			if err := res.Close(); err != nil {
				errs = append(errs, fmt.Errorf("resource close error: %w", err))
			}
		}
		l.resources = nil
	})

	return errors.Join(errs...)
}

func (l *Logger) SetLevel(level string) error {
	l.Logger.Info("log level changed", zap.String("level", level))
	return l.level.UnmarshalText([]byte(level))
}

func (l *Logger) NewHelper(keys ...interface{}) *log.Helper {
	return log.NewHelper(l.With(keys...))
}

func (l *Logger) With(keys ...interface{}) *Logger {
	fields := l.fieldPool.Get().([]zap.Field)
	defer func() {
		fields = fields[:0] // Reset slice
		l.fieldPool.Put(fields)
	}()

	for i := 0; i < len(keys); i += 2 {
		if i+1 >= len(keys) {
			fields = append(fields, zap.Any(fmt.Sprint(keys[i]), "(MISSING)"))
			continue
		}
		key, ok := keys[i].(string)
		if !ok {
			continue
		}
		fields = append(fields, zap.Any(key, keys[i+1]))
	}

	optsCopy := *l.config
	return &Logger{
		Logger:    l.Logger.With(fields...),
		config:    &optsCopy,
		level:     zap.NewAtomicLevelAt(l.level.Level()),
		resources: l.resources,
		fieldPool: l.fieldPool,
		closeOnce: sync.Once{},
	}
}
