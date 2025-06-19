package file

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	timeFormat        = "2006/01/02 15:04:05"
	defaultMaxSize    = 10 // 10 MB
	defaultMaxAge     = 7  // 7 days
	defaultMaxBackups = 3  // 3 back
)

// Log FileLog 单个文件的日志
type Log struct {
	logger *zap.Logger // zap 日志记录器
}

// NewFileLog 创建一个新的 TableLog
func NewFileLog(filename string) *Log {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeLevel = nil
	encoderCfg.EncodeTime = customTimeEncoder
	encoderCfg.ConsoleSeparator = " "
	fileEnc := zapcore.NewConsoleEncoder(encoderCfg)
	lj := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    defaultMaxSize,
		MaxAge:     defaultMaxAge,
		MaxBackups: defaultMaxBackups,
		LocalTime:  true,
		Compress:   true,
	}
	logger := zap.New(zapcore.NewTee(zapcore.NewCore(fileEnc, zapcore.AddSync(lj), zapcore.InfoLevel)))
	return &Log{
		logger: logger,
	}
}

// customTimeEncoder 自定义时间格式
func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString("[" + t.Format(timeFormat) + "]")
}

// Sync 确保日志被写入
func (l *Log) Sync() error {
	return l.logger.Sync()
}

// Infow 写入结构化日志
func (l *Log) Infow(msg string, kvs ...interface{}) {
	l.logger.Sugar().Infow(msg, kvs...)
}

// WriteLog 写入日志
func (l *Log) WriteLog(msg string, args ...interface{}) {
	l.logger.Sugar().Infof(msg, args...)
}

// userEnter 玩家进入游戏的日志记录
func (l *Log) userEnter(tableID, uid int64, seat int32, money int64) {
	l.WriteLog("<进入游戏> 玩家[%d %d] 金币[%d] 桌子号[%d]", uid, seat, money, tableID)
}
