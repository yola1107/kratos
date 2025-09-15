package zap

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/yola1107/kratos/v2/library/log/zap/conf"
)

const timeFormat = "2006/01/02 15:04:05.000"

var (
	levelColors = map[zapcore.Level]string{
		zapcore.DebugLevel:  "\x1b[36m",
		zapcore.InfoLevel:   "\x1b[32m",
		zapcore.WarnLevel:   "\x1b[33m",
		zapcore.ErrorLevel:  "\x1b[31m",
		zapcore.DPanicLevel: "\x1b[35m",
		zapcore.PanicLevel:  "\x1b[35m",
		zapcore.FatalLevel:  "\x1b[35m",
	}

	levelNames = map[zapcore.Level]string{
		zapcore.DebugLevel:  "DEBUG",
		zapcore.InfoLevel:   "INFO·",
		zapcore.WarnLevel:   "WARN·",
		zapcore.ErrorLevel:  "ERROR",
		zapcore.DPanicLevel: "PANIC",
		zapcore.PanicLevel:  "PANIC",
		zapcore.FatalLevel:  "FATAL",
	}
)

type zapWrap struct {
	log     *zap.Logger
	level   zap.AtomicLevel
	closers []io.Closer
}

func (w *zapWrap) close() error {
	_ = w.log.Sync()
	for _, closer := range w.closers {
		_ = closer.Close()
	}
	return nil
}

func newZapWrap(c *conf.Logger, alert *Alert) *zapWrap {
	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(c.Level)); err != nil {
		panic(fmt.Errorf("invalid log level: %s", c.Level))
	}

	cores := []zapcore.Core{
		newConsoleCore(level, false),
	}

	if alert != nil {
		cores = append(cores, alert)
	}

	if c.Mode == conf.MODE_PROD && c.Directory != "" {
		cores = append(cores, newFileCores(c, level)...)
	}

	opts := []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(zap.PanicLevel),
		zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return zapcore.NewSamplerWithOptions(
				core,
				time.Second,
				2000,
				10,
			)
		}),
	}

	return &zapWrap{
		log:     zap.New(zapcore.NewTee(cores...), opts...),
		level:   level,
		closers: getClosers(c, alert),
	}
}

func newConsoleCore(level zapcore.LevelEnabler, file bool) zapcore.Core {
	encoder := zapcore.NewConsoleEncoder(newEncoderConfig(file))
	return zapcore.NewCore(encoder, zapcore.Lock(os.Stderr), level)
}

func newFileCores(c *conf.Logger, level zap.AtomicLevel) []zapcore.Core {
	var cores []zapcore.Core
	app := getAppName(c)

	fileCore := func(filename string, minLevel zapcore.LevelEnabler) zapcore.Core {
		writer := &lumberjack.Logger{
			Filename:   filepath.Join(c.Directory, filename),
			MaxSize:    int(c.Rotate.MaxSizeMB),
			MaxBackups: int(c.Rotate.MaxBackups),
			MaxAge:     int(c.Rotate.MaxAgeDays),
			Compress:   c.Rotate.Compress,
			LocalTime:  c.Rotate.LocalTime,
		}

		encoder := zapcore.NewConsoleEncoder(newEncoderConfig(true))
		if c.FormatJson {
			encoder = zapcore.NewJSONEncoder(newEncoderConfig(true))
		}

		return zapcore.NewCore(encoder, zapcore.AddSync(writer), minLevel)
	}

	cores = append(cores, fileCore(app+".log", level))
	if c.ErrorFile {
		cores = append(cores, fileCore(app+"_error.log", zap.ErrorLevel))
	}

	return cores
}

func getAppName(c *conf.Logger) string {
	if c.AppName != "" {
		return c.AppName
	}
	return "app"
}

func getClosers(c *conf.Logger, alert *Alert) (closers []io.Closer) {
	if alert != nil {
		closers = append(closers, alert)
	}
	return closers
}

func newEncoderConfig(file bool) zapcore.EncoderConfig {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = timeEncoder
	cfg.EncodeLevel = levelEncoder
	cfg.EncodeCaller = callerEncoder
	cfg.ConsoleSeparator = " "

	if !file {
		cfg.EncodeLevel = colorLevelEncoder
		cfg.EncodeCaller = zapcore.FullCallerEncoder
	}
	return cfg
}

func timeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(fmt.Sprintf("[%s]", t.Format(timeFormat)))
}

func callerEncoder(c zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(fmt.Sprintf("[%s]", c.FullPath()))
}

func levelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(fmt.Sprintf("[%s]", levelNames[l]))
}

func colorLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	color := levelColors[l]
	enc.AppendString(fmt.Sprintf("[%s%s\x1b[0m]", color, levelNames[l]))
}
