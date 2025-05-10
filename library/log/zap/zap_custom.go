package zap

//
//import (
//	"fmt"
//	"time"
//
//	"go.uber.org/zap/zapcore"
//)
//
//func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
//	enc.AppendString(fmt.Sprintf("[%s]", t.Format("2006/01/02 15:04:05.000")))
//}
//
//func customLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
//	enc.AppendString(fmt.Sprintf("[%-5s]", l.CapitalString()))
//}
//
//func customColorLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
//	var color string
//	switch l {
//	case zapcore.DebugLevel:
//		color = "\x1b[36m" // Cyan
//	case zapcore.InfoLevel:
//		color = "\x1b[32m" // Green
//	case zapcore.WarnLevel:
//		color = "\x1b[33m" // Yellow
//	case zapcore.ErrorLevel:
//		color = "\x1b[31m" // Red
//	case zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
//		color = "\x1b[35m" // Magenta
//	default:
//		color = "\x1b[0m" // Reset
//	}
//	levelStr := fmt.Sprintf("[%s%s\x1b[0m]", color, l.CapitalString())
//	enc.AppendString(levelStr)
//}
//
//func customCallerEncoder(c zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
//	enc.AppendString(fmt.Sprintf("[%s]", c.FullPath()))
//}
