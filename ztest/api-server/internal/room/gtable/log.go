package gtable

import (
	"fmt"

	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/model"
)

const (
	TableLogDirPath = "./logs/log_cache/%s/table_%d.log"
)

type TableLog struct {
	tableID int32
	c       *conf.Room_LogCache
	logger  *model.FileLog
}

func (l *TableLog) init(tableID int32, c *conf.Room_LogCache) {
	l.c = c
	l.tableID = tableID
	l.logger = model.NewFileLog(fmt.Sprintf(TableLogDirPath, conf.Name, tableID))
}

func (l *TableLog) Close() error {
	return l.logger.Close()
}

// WriteLog 写入日志
func (l *TableLog) write(msg string, args ...interface{}) {
	if !l.c.Open {
		return
	}
	l.logger.WriteLog(msg, args...)
}

// UserEnter 玩家进入游戏的日志记录
func (l *TableLog) userEnter(uid int64, seat int32, money int64) {
	l.write("<进入游戏> 玩家[%d %d] 金币[%d] 桌子号[%d]", uid, seat, money, l.tableID)
}
