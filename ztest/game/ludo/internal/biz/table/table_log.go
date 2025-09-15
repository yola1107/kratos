package table

import (
	"fmt"
	"strings"

	"github.com/yola1107/kratos/v2/library/log/file"
	"github.com/yola1107/kratos/v2/library/xgo"
	v1 "github.com/yola1107/kratos/v2/ztest/game/ludo/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/model"
)

const (
	LogDirPath = "./logs/log_cache/%s/table_%d.log"
)

type Log struct {
	c       *conf.Room_LogCache
	tableID int32
	logger  *file.Log
}

func NewTableLog(tableID int32, c *conf.Room_LogCache) *Log {
	return &Log{
		c:       c,
		tableID: tableID,
		logger:  file.NewFileLog(fmt.Sprintf(LogDirPath, conf.Name, tableID)),
	}
}

func (l *Log) Close() error {
	return l.logger.Sync()
}

// WriteLog 写入到桌子日志文件
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

// 玩家离线
func (l *Log) offline(player *player.Player) {
	l.write("【玩家断线】玩家:%+v ", player.Desc())
}

func (l *Log) begin(tb string, bet float64, seats []*player.Player, infos any) {
	logs := []string{fmt.Sprintf("[游戏开始] %s bet:%.1f Gamer=%v", tb, bet, infos)}
	for _, p := range seats {
		if p == nil {
			continue
		}
		logs = append(logs, fmt.Sprintf("玩家:%+v 投注[%+v] color=%v 状态:%v", p.Desc(), bet, p.GetColor(), p.GetStatus()))
	}
	l.write(strings.Join(logs, "\r\n"))
}

func (l *Log) activePush(p *player.Player, canAction v1.ACTION_TYPE, canMoveStr, retStr string) {
	l.write("[操作通知] 玩家:%+v canAction=%q, canMoveDice=%s, Ret=%v", p.Desc(), canAction, canMoveStr, retStr)
}

func (l *Log) stage(s string, active int32) {
	l.write("[状态转移] %s. active=%+v", s, active)
}

func (l *Log) Dice(p *player.Player, dice int32, movable bool, timeout bool) {
	l.write("[玩家掷骰] 玩家:%+v. dice=%d, movable=%v, timeout=%+v", p.Desc(), dice, movable, timeout)
}

func (l *Log) Move(p *player.Player, pieceID, diceValue int32, isArrived bool, step *model.Step, timeout bool) {
	eat := []int32(nil)
	if step != nil && len(step.Killed) > 0 {
		for _, k := range step.Killed {
			eat = append(eat, k.Id)
		}
	}
	eatStr := ""
	if len(eat) > 0 {
		eatStr = fmt.Sprintf("(cnt=%d,e=%v)", len(eat), eat)
	}
	l.write("[玩家移动] 玩家:%+v. [id=%d, x=%d], isArrived=%v, eat=%s, step=%v, timeout=%+v",
		p.Desc(), pieceID, diceValue, isArrived, eatStr, xgo.ToJSON(step), timeout)
}

func logPlayers(players []*player.Player) string {
	logs := []string{""}
	for _, p := range players {
		if p == nil {
			continue
		}
		logs = append(logs, fmt.Sprintf("<玩家>:%+v 状态:%v", p.Desc(), p.GetStatus()))
	}
	return strings.Join(logs, "\r\n")
}

func (l *Log) settle(winner *player.Player, win, tax float64, msgs ...any) {
	logs := []string{"[结算]"}
	if winner != nil {
		logs = append(logs, fmt.Sprintf("<赢家>:%+v win:%.1f tax:%.1f color:%v",
			winner.Desc(), win, tax, winner.GetColor()))
	}
	for _, msg := range msgs {
		logs = append(logs, fmt.Sprintf("%v", msg))
	}
	l.write(strings.Join(logs, "\r\n"))
}

func (l *Log) endClear(msg ...any) {
	l.write("[结束后清理数据] %s", msg)
}

func (l *Log) end(msg ...any) {
	l.write("[GameEnd] %s", msg)
	l.write("\r\n\r\n\r\n")
}
