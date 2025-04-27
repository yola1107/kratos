package gtable

import (
	"fmt"

	"github.com/yola1107/kratos/v2/ztest/api-server/internal/model"
)

const (
	TableLogDirPath = "./logs/log_cache/%+v" // 日志目录 log/log_cache/20060102/
)

type tableLogger struct {
	tableID int32
	logger  *model.FileLog
}

func newTableLog(tableID int32) *tableLogger {
	filename := fmt.Sprintf(TableLogDirPath, tableID)
	return &tableLogger{
		tableID: tableID,
		logger:  model.NewFileLog(filename),
	}
}

func (l *tableLogger) Close() error {
	return l.logger.Close()
}

// WriteLog 写入日志
func (l *tableLogger) write(msg string, args ...interface{}) {
	// if !conf.GetGameConfig().LogCacheOpen {
	// 	return
	// }
	l.logger.WriteLog(msg, args...)
}

// UserEnter 玩家进入游戏的日志记录
func (l *tableLogger) UserEnter(uid int64, seat int32, money int64) {
	l.write("<进入游戏> 玩家[%d %d] 金币[%d] 桌子号[%d]", uid, seat, money, l.tableID)
}
