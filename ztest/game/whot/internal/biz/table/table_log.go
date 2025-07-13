package table

import (
	"fmt"
	"strings"

	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/library/log/file"
	v1 "github.com/yola1107/kratos/v2/ztest/game/whot/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/conf"
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
		logs = append(logs, fmt.Sprintf("玩家:%+v 投注[%+v] Hands:%v 状态:%v", p.Desc(), bet, p.GetCards(), p.GetStatus()))
	}
	l.write(strings.Join(logs, "\r\n"))
}

func (l *Log) activePush(p *player.Player, currCard int32, pending *v1.Pending, canOp []*v1.ActionOption) {
	l.write("[操作通知] 玩家:%+v curr=%d, pending=%v, canOp=%v", p.Desc(), currCard, descPendingEffect(pending), ext.ToJSON(canOp))
}

func (l *Log) stage(s string, active int32) {
	l.write("[状态转移] %s. active=%+v", s, active)
}

func (l *Log) play(p *player.Player, card int32, pending *v1.Pending, timeout bool) {
	l.write("[玩家出牌] 玩家:%+v. out=[%+v] pending=%s, timeout=%+v", p.Desc(), card, descPending(pending), timeout)
}

func (l *Log) draw(p *player.Player, card []int32, pending *v1.Pending, timeout bool) {
	l.write("[玩家抓牌] 玩家:%+v. drawn=%+v, pending=%s, timeout=%+v", p.Desc(), card, descPending(pending), timeout)
}

func (l *Log) replyPending(p *player.Player, action v1.ACTION, pending *v1.Pending) {
	l.write("[响应Pending] 玩家:%+v. action=%q 响应了pending，清除:%s", p.Desc(), action, descPending(pending))
}

func (l *Log) market(p *player.Player, card []int32, pending *v1.Pending, timeout bool) {
	l.write("[market各抽一张] 玩家:%+v. drawn=%+v, pending=%s, timeout=%+v", p.Desc(), card, descPending(pending), timeout)
}

func (l *Log) skipTurn(p *player.Player, timeout bool) {
	l.write("[suspend玩家跳过] 玩家:%+v pending=, timeout=%v", p.Desc(), timeout)
}

func (l *Log) declareSuit(p *player.Player, suit v1.SUIT, currCard int32, timeout bool) {
	l.write("[Whot确定花色] 玩家:%+v 花色=%d, currCard=%v, timeout=%+v", p.Desc(), suit, currCard, timeout)
}

func descPending(pending *v1.Pending) string {
	if pending == nil {
		return ""
	}
	return fmt.Sprintf("{%+v->%v %v %v} ",
		pending.Initiator, pending.Target, pending.Effect, pending.Quantity)
}

func descPendingEffect(pending *v1.Pending) string {
	if pending == nil {
		return ""
	}
	return fmt.Sprintf("%q", pending.Effect)
}

func logPlayers(players []*player.Player) string {
	logs := []string{""}
	for _, p := range players {
		if p == nil {
			continue
		}
		logs = append(logs, fmt.Sprintf("<玩家>:%+v Hands:%v 状态:%v", p.Desc(), p.GetCards(), p.GetStatus()))
	}
	return strings.Join(logs, "\r\n")
}

func (l *Log) settle(winner *player.Player, win, tax float64, msgs ...any) {
	logs := []string{"[结算]"}
	if winner != nil {
		logs = append(logs, fmt.Sprintf("<赢家>:%+v win:%.1f tax:%.1f Hands:%v",
			winner.Desc(), win, tax, winner.GetCards()))
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
