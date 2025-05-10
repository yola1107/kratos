package zap

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/yola1107/kratos/v2/log"
)

var _ log.Logger = (*Logger)(nil)

const (
	defaultFieldCapacity = 32
	maxPoolCapacity      = 1024
	sensitiveMask        = "******"
)

type Logger struct {
	*zap.Logger
	level         zap.AtomicLevel
	closer        *loggerCloser
	fieldPool     *fieldSlicePool
	sensitiveKeys map[string]struct{}
}

type loggerCloser struct {
	resources []io.Closer
	closeOnce sync.Once
}

// 优化的字段池实现
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

// NewLogger 创建日志实例
func NewLogger(opts ...Option) (*Logger, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// 初始化日志级别
	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level.SetLevel(zap.InfoLevel)
	}

	// 构建编码器和核心
	encoderConfig := buildEncoderConfig(cfg)
	cores, resources, err := createCore(cfg, level, encoderConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create log cores: %w", err)
	}

	// 配置采样
	var core zapcore.Core
	if cfg.Sampling != nil {
		core = zapcore.NewSamplerWithOptions(
			zapcore.NewTee(cores...),
			cfg.Sampling.Window,
			cfg.Sampling.Initial,
			cfg.Sampling.Thereafter,
		)
	} else {
		core = zapcore.NewTee(cores...)
	}

	options := []zap.Option{
		zap.AddCaller(),
		zap.AddCallerSkip(2),
		zap.AddStacktrace(zap.PanicLevel),
	}
	if cfg.Mode == Development {
		options = append(options, zap.Development())
	}

	closer := &loggerCloser{
		resources: resources,
	}

	logger := &Logger{
		Logger:        zap.New(core, options...),
		level:         level,
		closer:        closer,
		fieldPool:     newFieldSlicePool(),
		sensitiveKeys: make(map[string]struct{}),
	}

	// 初始化敏感词
	for _, k := range cfg.SensitiveKeys {
		logger.sensitiveKeys[strings.ToLower(k)] = struct{}{}
	}

	log.Infof("Zap logger initialized. mode=%s cores=%d conf=%+v", cfg.Mode, len(cores), cfg)
	return logger, nil
}

func buildEncoderConfig(cfg *Config) zapcore.EncoderConfig {
	encConfig := zapcore.EncoderConfig{
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
		encConfig.EncodeLevel = customColorLevelEncoder
		encConfig.EncodeCaller = zapcore.FullCallerEncoder
	}
	return encConfig
}

// createCore 创建日志核心
func createCore(cfg *Config, level zap.AtomicLevel, encoderConfig zapcore.EncoderConfig) ([]zapcore.Core, []io.Closer, error) {
	var (
		cores     []zapcore.Core
		resources []io.Closer
	)

	// 生产模式核心
	if cfg.Mode == Production {
		infoWriter, errWriter, err := createWriters(cfg)
		if err != nil {
			return nil, nil, err
		}
		resources = append(resources, infoWriter.(io.Closer), errWriter.(io.Closer))

		encoder := zapcore.NewConsoleEncoder(encoderConfig)
		cores = append(cores,
			zapcore.NewCore(encoder, zapcore.AddSync(infoWriter), level),
			zapcore.NewCore(encoder, zapcore.AddSync(errWriter), zap.ErrorLevel),
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

// 创建日志文件写入器（工厂模式）
func createWriters(cfg *Config) (info, error io.Writer, err error) {
	if err = os.MkdirAll(cfg.Directory, 0750); err != nil {
		return nil, nil, fmt.Errorf("create log directory failed: %w", err)
	}

	return newLumberjack(cfg, cfg.Filename),
		newLumberjack(cfg, cfg.ErrorFilename), nil
}

func newLumberjack(cfg *Config, filename string) io.Writer {
	return &lumberjack.Logger{
		Filename:   filepath.Join(cfg.Directory, filename),
		MaxSize:    cfg.MaxSize,
		MaxAge:     cfg.MaxAge,
		MaxBackups: cfg.MaxBackups,
		Compress:   cfg.Compress,
		LocalTime:  cfg.LocalTime,
	}
}

func createAlertCore(cfg *Config, encoderConfig zapcore.EncoderConfig) *Alerter {
	if cfg.Alert.Threshold == zapcore.InvalidLevel {
		return nil
	}

	alertEncoder := encoderConfig
	alertEncoder.EncodeLevel = zapcore.CapitalLevelEncoder
	alertEncoder.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000") //zapcore.TimeEncoderOfLayout(time.RFC3339)
	alertEncoder.EncodeCaller = zapcore.FullCallerEncoder

	sender, err := NewTelegramSender(cfg.Alert.Notification.Telegram)
	if err != nil {
		log.Warnf("Failed to create Alerter: %v", err)
		return nil
	}

	return NewAlerter(
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= cfg.Alert.Threshold
		}),
		zapcore.NewJSONEncoder(alertEncoder),
		cfg.Alert,
		sender,
	)
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

// Close 关闭
func (l *Logger) Close() error {
	var errs []error

	l.closer.closeOnce.Do(func() {
		if err := l.Sync(); err != nil && !isBenignSyncError(err) {
			errs = append(errs, fmt.Errorf("logger sync error: %w", err))
		}

		for i, res := range l.closer.resources {
			if err := res.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close resource[%d] %T failed: %w",
					i, res, err))
			}
		}
		l.closer.resources = nil
		log.Info("Zap Logger closed successfully")
	})

	return errors.Join(errs...)
}

// isBenignSyncError 判断是否为可忽略的同步错误
func isBenignSyncError(err error) bool {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		if pathErr.Op == "sync" {
			switch pathErr.Path {
			case "/dev/stdout", "/dev/stderr":
				return true
			}
		}
	}
	return false
}

func (l *Logger) SetLevel(level string) error {
	return l.level.UnmarshalText([]byte(level))
}

func (l *Logger) NewHelper(keys ...interface{}) *log.Helper {
	return log.NewHelper(l.With(keys...))
}

// With 方法创建子Logger，共享资源管理器
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
			key = fmt.Sprintf("%v", keys[i])
		}
		fields = append(fields, zap.Any(key, keys[i+1]))
	}

	return &Logger{
		Logger:        l.Logger.With(fields...),
		level:         l.level,
		closer:        l.closer,
		fieldPool:     l.fieldPool,
		sensitiveKeys: l.sensitiveKeys,
	}
}
