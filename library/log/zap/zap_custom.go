package zap

import (
	"fmt"
	"time"

	"go.uber.org/zap/zapcore"
)

// 颜色常量（ANSI escape codes）
var (
	_resetColor   = "\x1b[0m" // Reset
	_levelToColor = map[zapcore.Level]string{
		zapcore.DebugLevel:  "\x1b[36m", // Cyan
		zapcore.InfoLevel:   "\x1b[32m", // Green
		zapcore.WarnLevel:   "\x1b[33m", // Yellow
		zapcore.ErrorLevel:  "\x1b[31m", // Red
		zapcore.DPanicLevel: "\x1b[35m", // Magenta
		zapcore.PanicLevel:  "\x1b[35m", // Magenta
		zapcore.FatalLevel:  "\x1b[35m", // Magenta
	}
)

func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(fmt.Sprintf("[%s]", t.Format("2006/01/02 15:04:05.000")))
}

func customLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(fmt.Sprintf("[%-5s]", l.CapitalString()))
}

func customColorLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	c, ok := _levelToColor[l]
	if !ok {
		c = _resetColor
	}
	enc.AppendString("[" + c + l.CapitalString() + "\x1b[0m]")
}

func customCallerEncoder(c zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(fmt.Sprintf("[%s]", c.FullPath()))
}
