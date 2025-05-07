package zap

import (
	"fmt"
	"io"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/yola1107/kratos/v2/log"
)

type Mode int32

const (
	Development Mode = 0
	Production  Mode = 1
)

type Config struct {
	Mode          Mode   `yaml:"mode" json:"mode"`
	Level         string `yaml:"level" json:"level"`
	Directory     string `yaml:"directory" json:"directory"`
	Filename      string `yaml:"filename" json:"filename"`
	ErrorFilename string `yaml:"error_filename" json:"error_filename"`
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
	}
	return nil, nil
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
