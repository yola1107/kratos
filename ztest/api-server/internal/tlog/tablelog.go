package tlog

import (
	"fmt"
	"io"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	timeFormat = "2006/01/02 15:04:05"
)

// TableMgr 管理多个桌子的日志
type TableMgr struct {
	Tables map[int64]*TableLog
}

// TableLog 单个桌子的日志
type TableLog struct {
	id      int64
	closers io.Closer
	logger  *zap.Logger // zap 日志记录器
	enable  bool        // write enable
}

// NewTableLog 创建一个新的 TableLog
func NewTableLog(id int64, enable bool) *TableLog {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeLevel = nil
	encoderCfg.EncodeTime = customTimeEncoder
	encoderCfg.ConsoleSeparator = " "
	fileEnc := zapcore.NewConsoleEncoder(encoderCfg)
	lj := &lumberjack.Logger{
		Filename:   fmt.Sprintf("./logs/table_%d.log", id),
		MaxSize:    200,
		MaxAge:     7,
		MaxBackups: 2,
		LocalTime:  true,
		Compress:   true,
	}
	logger := zap.New(zapcore.NewTee(zapcore.NewCore(fileEnc, zapcore.AddSync(lj), zapcore.InfoLevel)))
	return &TableLog{
		id:      id,
		closers: lj,
		logger:  logger,
		enable:  enable,
	}
}

// Sync 确保日志被写入
func (l *TableLog) Sync() error {
	return l.logger.Sync()
}

// Close 关闭日志资源
func (l *TableLog) Close() error {
	_ = l.Sync()
	return l.closers.Close()
}

func (l *TableLog) SetEnable(enable bool) {
	l.enable = enable
}

// WriteLog 写入日志
func (l *TableLog) WriteLog(msg string, args ...interface{}) {
	if !l.enable {
		return
	}
	l.logger.Sugar().Infof(msg, args...)
}

// customTimeEncoder 自定义时间格式
func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(fmt.Sprintf("[%s]", t.Format(timeFormat)))
}

// UserEnter 玩家进入游戏的日志记录
func (l *TableLog) UserEnter(uid int64, seat int32, money int64) {
	l.WriteLog("【进入游戏】玩家[%d %d] 金币[%d] 桌子号[%d]", uid, seat, money, l.id)
}
