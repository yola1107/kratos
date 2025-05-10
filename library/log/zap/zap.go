package zap

import (
	"errors"
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
	defaultFieldCapacity = 16
	sensitiveMask        = "******"
)

type Logger struct {
	*zap.Logger
	level     zap.AtomicLevel
	resources []io.Closer
	fieldPool *fieldSlicePool
	closeOnce sync.Once
	sampler   zapcore.Core // 采样核心
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
	return p.pool.Get().([]zap.Field)
}

func (p *fieldSlicePool) Put(fields []zap.Field) {
	fields = fields[:0]
	p.pool.Put(fields)
}

// NewLogger creates a new Logger instance. Panics on initialization errors.
func NewLogger(opts ...Option) (*Logger, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// 设置日志级别
	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level.SetLevel(zap.InfoLevel)
	}

	// 创建编码器
	encoderConfig := buildEncoderConfig(cfg)
	cores, resources, err := createCore(cfg, level, encoderConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create log cores: %w", err)
	}

	var sampler zapcore.Core
	if cfg.Sampling != nil {
		sampler = zapcore.NewSampler(
			zapcore.NewTee(cores...),
			time.Second,
			cfg.Sampling.Initial,
			cfg.Sampling.Thereafter,
		)
	}

	options := []zap.Option{
		zap.AddCaller(),
		zap.AddCallerSkip(2),
		zap.AddStacktrace(zap.PanicLevel),
	}

	if cfg.Mode == Development {
		options = append(options, zap.Development())
	}

	var core zapcore.Core
	if sampler != nil {
		core = sampler
	} else {
		core = zapcore.NewTee(cores...)
	}

	logger := &Logger{
		Logger:    zap.New(core, options...),
		level:     level,
		resources: resources,
		fieldPool: newFieldSlicePool(),
	}
	log.Infof("zap logger initialized with config: %+v", cfg)
	return logger, nil
}

func buildEncoderConfig(cfg *Config) zapcore.EncoderConfig {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:          "ts",
		LevelKey:         "level",
		NameKey:          "logger",
		CallerKey:        "caller",
		FunctionKey:      zapcore.OmitKey,
		MessageKey:       "msg",
		StacktraceKey:    "stack",
		LineEnding:       zapcore.DefaultLineEnding,
		EncodeLevel:      customLevelEncoder,
		EncodeTime:       customTimeEncoder,
		EncodeDuration:   zapcore.StringDurationEncoder,
		EncodeCaller:     customCallerEncoder,
		ConsoleSeparator: " ",
	}

	if cfg.Mode == Development {
		encoderConfig.EncodeLevel = customColorLevelEncoder
		encoderConfig.EncodeCaller = zapcore.FullCallerEncoder
	}

	return encoderConfig
}

// 创建核心日志器
func createCore(cfg *Config, level zap.AtomicLevel, encoderConfig zapcore.EncoderConfig) ([]zapcore.Core, []io.Closer, error) {
	var (
		cores     []zapcore.Core
		resources []io.Closer
	)

	// 生产模式核心
	if cfg.Mode == Production {
		infoWriter, errorWriter, err := buildWriters(cfg)
		if err != nil {
			return nil, nil, err
		}
		resources = append(resources, infoWriter.(io.Closer), errorWriter.(io.Closer))

		encoder := zapcore.NewConsoleEncoder(encoderConfig)
		cores = append(cores,
			zapcore.NewCore(encoder, zapcore.AddSync(infoWriter), level),
			zapcore.NewCore(encoder, zapcore.AddSync(errorWriter), zap.ErrorLevel),
		)
	}

	// 告警核心
	if alerter := createAlertCore(cfg, encoderConfig); alerter != nil {
		resources = append(resources, alerter)
		cores = append(cores, alerter)
	}

	// 控制台核心
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	cores = append(cores, zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stderr), level))

	return cores, resources, nil
}

func createAlertCore(cfg *Config, encoderConfig zapcore.EncoderConfig) *Alerter {
	if cfg.Alert.Threshold == zapcore.InvalidLevel {
		return nil
	}

	tgEncoder := encoderConfig
	tgEncoder.EncodeLevel = zapcore.CapitalLevelEncoder
	tgEncoder.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
	tgEncoder.EncodeCaller = zapcore.FullCallerEncoder

	return NewAlerter(
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= cfg.Alert.Threshold
		}),
		zapcore.NewJSONEncoder(tgEncoder),
		cfg.Alert,
	)
}

func buildWriters(cfg *Config) (infoWriter, errorWriter io.Writer, err error) {
	if err := os.MkdirAll(cfg.Directory, 0750); err != nil {
		return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	infoWriter = &lumberjack.Logger{
		Filename:   filepath.Join(cfg.Directory, cfg.Filename),
		MaxSize:    cfg.MaxSize,
		MaxAge:     cfg.MaxAge,
		MaxBackups: cfg.MaxBackups,
		Compress:   cfg.Compress,
		LocalTime:  cfg.LocalTime,
	}

	errorWriter = &lumberjack.Logger{
		Filename:   filepath.Join(cfg.Directory, cfg.ErrorFilename),
		MaxSize:    cfg.MaxSize,
		MaxAge:     cfg.MaxAge,
		MaxBackups: cfg.MaxBackups,
		Compress:   cfg.Compress,
		LocalTime:  cfg.LocalTime,
	}

	return infoWriter, errorWriter, nil
}

func (l *Logger) Log(level log.Level, keyvals ...interface{}) error {
	if len(keyvals) == 0 {
		return nil
	}

	var msg string
	fields := l.fieldPool.Get()
	defer l.fieldPool.Put(fields)

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
	sensitiveKeys := []string{"password", "token", "secret", "authorization", "creditcard"}
	for i, field := range fields {
		key := strings.ToLower(field.Key)
		for _, sk := range sensitiveKeys {
			if strings.Contains(key, sk) {
				fields[i] = zap.String(field.Key, sensitiveMask)
				break
			}
		}
	}
	return fields
}

// Close 同步关闭方法
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
		log.Infof("zap logger closed")
	})

	return errors.Join(errs...)
}

func (l *Logger) SetLevel(level string) error {
	return l.level.UnmarshalText([]byte(level))
}

func (l *Logger) NewHelper(keys ...interface{}) *log.Helper {
	return log.NewHelper(l.With(keys...))
}

func (l *Logger) With(keys ...interface{}) log.Logger {
	fields := l.fieldPool.Get()
	defer l.fieldPool.Put(fields)

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

	return &Logger{
		Logger:    l.Logger.With(fields...),
		level:     l.level,
		resources: l.resources,
		fieldPool: l.fieldPool,
		closeOnce: sync.Once{},
	}
}
