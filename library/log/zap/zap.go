package zap

import (
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/yola1107/kratos/v2/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/yola1107/kratos/v2/library/log/zap/conf"
)

var _ log.Logger = (*Logger)(nil)

const (
	sensitiveMask = "***"
)

type Logger struct {
	wrap       *zapWrap
	sensitives map[string]struct{}
	mu         sync.RWMutex
}

func NewLogger(c *conf.Bootstrap) *Logger {
	if c == nil {
		c = conf.DefaultConfig()
	}
	return wireLogger(c)
}

func initLogger(c *conf.Logger, wrap *zapWrap) *Logger {
	l := &Logger{
		wrap:       wrap,
		sensitives: make(map[string]struct{}),
	}
	l.SetSensitive(c.Sensitive)
	log.Debugf("Logger initialized. mode:%d app:%q level:%q directory:%q sensitives:%v",
		c.Mode, c.AppName, c.Level, c.Directory, c.Sensitive)
	return l
}

func (l *Logger) Log(level log.Level, keyvals ...any) error {
	// If logging at this level is completely disabled, skip the overhead of
	// string formatting.
	if zapcore.Level(level) < zapcore.DPanicLevel && !l.wrap.log.Core().Enabled(zapcore.Level(level)) {
		return nil
	}

	var (
		msg    = ""
		keylen = len(keyvals)
	)

	if keylen == 0 || keylen%2 != 0 {
		l.wrap.log.Warn(fmt.Sprint("Keyvalues must appear in pairs: ", keyvals))
		return nil
	}

	fields := make([]zap.Field, 0, (keylen/2)+1)
	for i := 0; i < keylen; i += 2 {
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

	logger := l.wrap.log.WithOptions(zap.AddCallerSkip(calculateSkip()))

	switch level {
	case log.LevelDebug:
		logger.Debug(msg, fields...)
	case log.LevelInfo:
		logger.Info(msg, fields...)
	case log.LevelWarn:
		logger.Warn(msg, fields...)
	case log.LevelError:
		logger.Error(msg, fields...)
	case log.LevelFatal:
		logger.Fatal(msg, fields...)
	}
	return nil
}

func (l *Logger) Close() error {
	defer l.wrap.log.Info("logger closed successfully")
	return l.wrap.close()
}

func (l *Logger) GetZap() *zap.Logger {
	return l.wrap.log
}

func (l *Logger) GetLevel() string {
	return l.wrap.level.String()
}

func (l *Logger) SetLevel(level string) {
	if err := l.wrap.level.UnmarshalText([]byte(level)); err != nil {
		l.wrap.log.Info("invalid log level",
			zap.String("level", level),
			zap.Error(err))
		return
	}
	l.wrap.log.Info("log level updated", zap.String("level", level))
}

func (l *Logger) GetSensitive() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	keys := make([]string, 0, len(l.sensitives))
	for k := range l.sensitives {
		keys = append(keys, k)
	}
	return keys
}

func (l *Logger) SetSensitive(keys []string) {
	set := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		set[strings.ToLower(k)] = struct{}{}
	}

	l.mu.Lock()
	l.sensitives = set
	l.mu.Unlock()
}

func (l *Logger) filterSensitive(fields []zap.Field) []zap.Field {
	if len(l.sensitives) == 0 {
		return fields
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	for i, field := range fields {
		if _, ok := l.sensitives[strings.ToLower(field.Key)]; ok {
			fields[i] = zap.String(field.Key, sensitiveMask)
		}
	}

	return fields
}

func calculateSkip() int {
	pc := make([]uintptr, 8)
	n := runtime.Callers(3, pc) // 调整跳过层数
	if n == 0 {
		return 2
	}

	frames := runtime.CallersFrames(pc[:n])

	for frame, more := frames.Next(); more; frame, more = frames.Next() {
		if strings.Contains(frame.Function, "kratos/v2/log.(*") {
			return 3 // Kratos 日志框架额外跳过
		}
	}
	return 2
}
