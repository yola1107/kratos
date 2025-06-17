package gtable

import (
	"fmt"

	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/entity/gplayer"
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
	return l.logger.Sync()
}

// WriteLog 写入日志
func (l *TableLog) write(msg string, args ...interface{}) {
	if !l.c.Open {
		return
	}
	l.logger.WriteLog(msg, args...)
}

// UserEnter 玩家进入游戏的日志记录
func (l *TableLog) userEnter(p *gplayer.Player, sitCnt int16) {
	l.write("[进入房间] 玩家:%+v 桌子人数(%+v) ", p.Desc(), sitCnt)
}

func (l *TableLog) userReEnter(p *gplayer.Player, sitCnt int16) {
	l.write("[重进房间] 玩家:%+v 桌子人数(%+v) ", p.Desc(), sitCnt)
}

func (l *TableLog) userExit(p *gplayer.Player, sitCnt int16, isSwitchTable bool) {
	l.write("[离开房间] 玩家:%+v 桌子人数(%+v) 是否换桌(%+v) ", p.Desc(), sitCnt, isSwitchTable)
}

func (l *TableLog) stage(old, new int32, active int) {
	l.write("【状态转移】[%v->%+v, %+v->%v]. activeChair:%+v",
		old, new, conf.StageNames[old], conf.StageNames[new], active)
}
