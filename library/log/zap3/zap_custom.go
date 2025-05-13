package zap

import (
	"fmt"
	"time"

	"go.uber.org/zap/zapcore"
)

// 常量定义
const (
	timeFormat = "2006/01/02 15:04:05.000"
)

// 颜色常量（ANSI escape codes）
var (
	resetColor  = "\x1b[0m"
	levelColors = map[zapcore.Level]string{
		zapcore.DebugLevel:  "\x1b[36m", // Cyan
		zapcore.InfoLevel:   "\x1b[32m", // Green
		zapcore.WarnLevel:   "\x1b[33m", // Yellow
		zapcore.ErrorLevel:  "\x1b[31m", // Red
		zapcore.DPanicLevel: "\x1b[35m", // Magenta
		zapcore.PanicLevel:  "\x1b[35m", // Magenta
		zapcore.FatalLevel:  "\x1b[35m", // Magenta
	}

	// 预格式化的级别字符串
	levelStrings = map[zapcore.Level]string{
		zapcore.DebugLevel:  "DEBUG",
		zapcore.InfoLevel:   "INFO",
		zapcore.WarnLevel:   "WARN",
		zapcore.ErrorLevel:  "ERROR",
		zapcore.DPanicLevel: "PANIC",
		zapcore.PanicLevel:  "PANIC",
		zapcore.FatalLevel:  "FATAL",
	}
)

func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(fmt.Sprintf("[%s]", t.Format(timeFormat)))
}

func customCallerEncoder(c zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(fmt.Sprintf("[%s]", c.FullPath()))
}

func customLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(fmt.Sprintf("[%s]", levelStrings[l]))
}

func customColorLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	color, ok := levelColors[l]
	if !ok {
		color = resetColor
	}
	enc.AppendString(fmt.Sprintf("[%s%s%s]", color, levelStrings[l], resetColor))
}
