package table

import (
	"fmt"
	"strings"

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

func (l *Log) begin(sitCnt int16, banker, first int32, bet float64, chairs []int32, seats []*player.Player) {
	logs := []string{fmt.Sprintf("【游戏开始】 sitCnt:%d/%d banker:%d first:%d bet:%.1f 玩家椅子顺序:%d",
		sitCnt, len(chairs), banker, first, bet, chairs)}
	for _, p := range seats {
		logs = append(logs, fmt.Sprintf("玩家:%+v 投注[%+v]", p.Desc(), bet))
	}
	l.write(strings.Join(logs, "\r\n"))
}

func (l *Log) activePush(p *player.Player, first int32, curRound int32) {
	l.write("【活动玩家通知】 玩家:%+v first:%+v curRound:%d", p.Desc(), first, curRound)
}

func (l *Log) stage(old, new int32, active int32) {
	l.write("【状态转移】[%v->%+v, %+v->%v]. activeChair:%+v",
		old, new, StageNames[old], StageNames[new], active)
}

func (l *Log) SeeCard(p *player.Player) {
	l.write("【看牌】 玩家:%+v autoSee(%+v) ", p.Desc(), p.IsAutoCall())
}

func (l *Log) PackCard(p *player.Player, timeOut bool) {
	l.write("【丢牌】 玩家:%+v timeOut(%+v) ", p.Desc(), timeOut)
}

func (l *Log) CallCard(p *player.Player, bet float64, double bool) {
	l.write("【跟注】 玩家:%+v 加倍(%+v) bet:%.3f ", p.Desc(), double, bet)
}

func (l *Log) ShowCard(p, target *player.Player, bet float64) {
	l.write(fmt.Sprintf("【比牌】比牌金额:%.3f 发起玩家:%+v -> 目标玩家:%+v ", bet, p.Desc(), target.Desc()))
}

func (l *Log) SidedShow(p, target *player.Player, bet float64) {
	l.write(fmt.Sprintf("【提前比牌(发起)】比牌金额:%.3f 发起玩家:%+v -> 目标玩家:%+v ", bet, p.Desc(), target.Desc()))
}

func (l *Log) SideShowReply(p, target *player.Player, allow bool) {
	l.write(fmt.Sprintf("【提前比牌应答】allow:%+v 应答玩家:%+v -> 发起玩家:%+v ", allow, p.Desc(), target.Desc()))
}

func (l *Log) compareCard(kind CompareType, winner *player.Player, loss []*player.Player) {
	logs := []string{fmt.Sprintf("<比牌信息> kind=%v ", kind)}
	logs = append(logs, fmt.Sprintf("<赢家>:%+v Hands:%v", winner.Desc(), winner.DescHand()))
	for _, p := range loss {
		logs = append(logs, fmt.Sprintf("<输家>:%+v Hands:%v", p.Desc(), p.DescHand()))
	}
	l.write(strings.Join(logs, "\r\n\t\t"))
}

func (l *Log) settle(msg ...any) {
	logs := []string{"【结算】"}
	l.write(strings.Join(logs, "\r\n"))
}
func (l *Log) endClear(msg ...any) {
	l.write("【结束后清理数据】 %s", msg)
}
func (l *Log) end(msg ...any) {
	l.write("【GameEnd】 %s", msg)
	l.write("\r\n\r\n\r\n")
}
