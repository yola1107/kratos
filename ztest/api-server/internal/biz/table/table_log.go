package table

import (
	"fmt"
	"strings"

	"github.com/yola1107/kratos/v2/library/log/file"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
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
		logs = append(logs, fmt.Sprintf("玩家:%+v 投注[%+v] Hands:%v 状态:%v", p.Desc(), bet, p.GetHand(), p.GetStatus()))
	}
	l.write(strings.Join(logs, "\r\n"))
}

func (l *Log) activePush(p *player.Player, first int32, curRound int32, canOp []v1.ACTION, gamingCnt int) {
	l.write("[操作通知] 玩家:%+v first:%+v curRound:%d canOp:%v gaming:%d", p.Desc(), first, curRound, canOp, gamingCnt)
}

func (l *Log) stage(s string, active int32) {
	l.write("[状态转移] %s. active=%+v", s, active)
}

func (l *Log) SeeCard(p *player.Player) {
	l.write("[看牌] 玩家:%+v autoSee(%+v) ", p.Desc(), p.IsAutoCall())
}

func (l *Log) PackCard(p *player.Player, timeout bool) {
	l.write("[丢牌] 玩家:%+v timeout=%+v ", p.Desc(), timeout)
}

func (l *Log) CallCard(p *player.Player, bet float64, double bool, timeout bool) {
	l.write("[跟注] 玩家:%+v 加倍(%+v) bet:%.1f timeout=%+v", p.Desc(), double, bet, timeout)
}

func (l *Log) ShowCard(p, target *player.Player, bet float64, timeout bool) {
	l.write(fmt.Sprintf("[比牌] 比牌金额:%.1f 发起玩家:%+v -> 目标玩家:%+v timeout=%+v", bet, p.Desc(), target.Desc(), timeout))
}

func (l *Log) SidedShow(p, target *player.Player, bet float64, timeout bool) {
	l.write(fmt.Sprintf("[提前比牌(发起)] 比牌金额:%.1f 发起玩家:%+v -> 目标玩家:%+v timeout=%+v", bet, p.Desc(), target.Desc(), timeout))
}

func (l *Log) SideShowReply(p, target *player.Player, allow bool, timeout bool) {
	l.write(fmt.Sprintf("[提前比牌应答] allow:%+v 应答玩家:%+v -> 发起玩家:%+v timeout=%+v", allow, p.Desc(), target.Desc(), timeout))
}

func (l *Log) compareCard(kind CompareType, winner *player.Player, loss []*player.Player) {
	l.write(logCompare(kind, winner, loss))
}

func logCompare(kind CompareType, winner *player.Player, loss []*player.Player) string {
	logs := []string{fmt.Sprintf("<比牌信息> kind=%v ", kind)}
	logs = append(logs, fmt.Sprintf("<赢家>:%+v Hands:%v", winner.Desc(), winner.GetHand()))
	for _, p := range loss {
		logs = append(logs, fmt.Sprintf("<输家>:%+v Hands:%v", p.Desc(), p.GetHand()))
	}
	return strings.Join(logs, "\r\n\t\t")
}

func logPlayers(players []*player.Player) string {
	logs := []string{""}
	for _, p := range players {
		if p == nil {
			continue
		}
		logs = append(logs, fmt.Sprintf("<玩家>:%+v Hands:%v 状态:%v", p.Desc(), p.GetHand(), p.GetStatus()))
	}
	return strings.Join(logs, "\r\n")
}

func (l *Log) settle(winner *player.Player, msgs ...any) {
	logs := []string{"[结算]"}
	if winner != nil {
		logs = append(logs, fmt.Sprintf("<赢家>:%+v Hands:%v", winner.Desc(), winner.GetHand()))
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
