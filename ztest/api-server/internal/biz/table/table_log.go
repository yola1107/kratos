package table

import (
	"fmt"

	"github.com/yola1107/kratos/v2/library/log/file"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
)

const (
	LogDirPath = "./logs/log_cache/%s/table_%d.log"
)

type Log struct {
	tableID int32
	c       *conf.Room_LogCache
	logger  *file.Log
}

func (l *Log) init(tableID int32, c *conf.Room_LogCache) {
	l.c = c
	l.tableID = tableID
	l.logger = file.NewFileLog(fmt.Sprintf(LogDirPath, conf.Name, tableID))
}

func (l *Log) Close() error {
	return l.logger.Sync()
}

// WriteLog 写入日志
func (l *Log) write(msg string, args ...interface{}) {
	if !l.c.Open {
		return
	}
	l.logger.WriteLog(msg, args...)
}

// UserEnter 玩家进入游戏的日志记录
func (l *Log) userEnter(p *player.Player, sitCnt int16) {
	l.write("[进入房间] 玩家:%+v 桌子人数(%+v) ", p.Desc(), sitCnt)
}

func (l *Log) userReEnter(p *player.Player, sitCnt int16) {
	l.write("[重进房间] 玩家:%+v 桌子人数(%+v) ", p.Desc(), sitCnt)
}

func (l *Log) userExit(p *player.Player, sitCnt int16, lastChair int32, isSwitchTable bool) {
	l.write("[离开房间] 玩家:%+v 桌子人数(%+v) lastChair(%d) 是否换桌(%+v) ", p.Desc(), sitCnt, lastChair, isSwitchTable)
}

func (l *Log) stage(old, new int32, active int) {
	l.write("【状态转移】[%v->%+v, %+v->%v]. activeChair:%+v",
		old, new, conf.StageNames[old], conf.StageNames[new], active)
}
