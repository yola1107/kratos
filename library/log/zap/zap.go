package zap

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

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

type Logger struct {
	*zap.Logger
	level     zap.AtomicLevel
	resources []io.Closer
	fieldPool *sync.Pool
	closeOnce sync.Once
}

// New creates a new Logger instance. Panics on initialization errors.
func New(cfg *Config) *Logger {
	logger, err := newZapLogger(cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %v", err))
	}
	return logger
}

// NewWithError creates a new Logger instance and returns any initialization error.
func NewWithError(cfg *Config) (*Logger, error) {
	return newZapLogger(cfg)
}

func newZapLogger(cfg *Config) (*Logger, error) {
	if cfg == nil {
		cfg = DefaultConfig()
		log.Warnf("logger config is empty, use default config")
	}

	// 设置日志级别
	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level.SetLevel(zap.InfoLevel)
	}

	// 创建编码器
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:          "ts",
		LevelKey:         "level",
		NameKey:          "logger",
		CallerKey:        "caller",
		FunctionKey:      zapcore.OmitKey,
		MessageKey:       "msg",
		StacktraceKey:    "stack",
		LineEnding:       zapcore.DefaultLineEnding,
		EncodeLevel:      zapcore.CapitalLevelEncoder, // zapcore.LowercaseLevelEncoder,
		EncodeTime:       zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
		EncodeDuration:   zapcore.StringDurationEncoder,
		EncodeCaller:     zapcore.FullCallerEncoder,
		ConsoleSeparator: " ",
	}
	if cfg.Mode == Development {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderConfig.EncodeCaller = zapcore.FullCallerEncoder
	}

	cores, resources, err := createCore(cfg, level, encoderConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create log cores: %w", err)
	}

	return &Logger{
		Logger: zap.New(
			zapcore.NewTee(cores...),
			zap.AddCaller(),
			zap.AddCallerSkip(2),
			zap.AddStacktrace(zap.PanicLevel),
		),
		level:     level,
		resources: resources,
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
		if err := os.MkdirAll(cfg.Directory, 0750); err != nil {
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

		encoder := zapcore.NewConsoleEncoder(encoderConfig)
		cores = append(cores,
			zapcore.NewCore(encoder, zapcore.AddSync(infoWriter), level),
			zapcore.NewCore(encoder, zapcore.AddSync(errorWriter), zap.ErrorLevel),
		)
	}

	//alert core
	if alerter := NewAlerter(
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool { return lvl >= cfg.Alert.Threshold }),
		zapcore.NewConsoleEncoder(encoderConfig),
		cfg.Alert); alerter != nil {
		cores = append(cores, alerter)
		resources = append(resources, alerter)
	}

	// Always add console core
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	cores = append(cores, zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stderr), level))

	return cores, resources, nil
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

		if key == log.DefaultMessageKey {
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
	return l.level.UnmarshalText([]byte(level))
}

func (l *Logger) NewHelper(keys ...interface{}) *log.Helper {
	return log.NewHelper(l.With(keys...))
}

func (l *Logger) With(keys ...interface{}) log.Logger {
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

	return &Logger{
		Logger:    l.Logger.With(fields...),
		level:     l.level,
		resources: l.resources,
		fieldPool: l.fieldPool,
		closeOnce: sync.Once{},
	}
}
