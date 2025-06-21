package table

import (
	"fmt"
	"time"

	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
)

/*
	游戏主逻辑
*/

type Stage struct {
	state     int32         // 当前阶段
	prev      int32         // 上一阶段
	timerID   int64         // 阶段定时器ID
	startTime time.Time     // 阶段开始时间
	duration  time.Duration // 阶段持续时间
}

func (t *Table) OnTimer() {
	// log.Debugf("TimeOut Stage ... %s ", descState(t.stage.state))

	switch t.stage.state {
	case StReady:
		t.onGameStart()
	case StSendCard:
		t.onSendCardTimeout()
	case StAction: // 超时操作
		t.onActionTimeout()
	case StSideShow: // 发起提前比牌 等待应答
		t.onSideShowTimeout()
	case StSideShowAni:
		t.onSideShowAniTimeout()
	case StWaitEnd:
		t.gameEnd()
	case StEnd:
		t.onEndTimeout()
	default:
		log.Warnf("unhandled default case")
	}
}

// 计算当前阶段剩余时间
func (t *Table) calcRemainingTime() time.Duration {
	remain := t.stage.duration - time.Since(t.stage.startTime)
	return max(remain, time.Millisecond)
}

func (t *Table) updateStage(s int32) {
	timer := t.repo.GetTimer()
	timer.Cancel(t.stage.timerID) // 取消当前阶段的定时任务

	t.stage.prev = t.stage.state
	t.stage.state = s
	t.stage.startTime = time.Now()
	t.stage.duration = t.checkResetDuration(s)
	t.stage.timerID = timer.Once(t.stage.duration, t.OnTimer)
	t.mLog.stage(t.stage.prev, s, t.active)
	log.Debugf("Stage Changed.  %s -> %s ", descState(t.stage.prev), descState(t.stage.state))
}

func (t *Table) checkResetDuration(s int32) time.Duration {
	timeout := GetStageTimeout(s)
	// 检查是否调整超时时间
	return time.Duration(timeout) * time.Second
}

func (t *Table) checkReady() {
	okCnt := int16(0)
	autoReady := t.repo.GetRoomConfig().Game.AutoReady
	t.RangePlayer(func(k int32, p *player.Player) bool {
		if autoReady {
			p.SetStatus(player.StReady)
		}
		if p.IsReady() && p.GetAllMoney() >= t.curBet {
			okCnt++
		}
		return true
	})
	canStart := okCnt >= 2
	if !canStart {
		t.stage.state = StWait
		return
	}

	// 准备状态倒计时2s
	t.updateStage(StReady)
}

func (t *Table) onGameStart() {
	can, canGameSeats, chairs := t.checkStart()
	if !can {
		t.stage.state = StWait
		return
	}

	// 扣钱
	t.intoGaming(canGameSeats)

	// 计算庄家及操作玩家
	t.calcBanker()

	// 发牌
	t.dispatchCard(canGameSeats)

	// 发牌状态倒计时3s
	t.updateStage(StSendCard)

	log.Debugf("******** <游戏开始> banker:%d first:%d currBet:%.1f sitCnt:%d GamingCnt:%d canGameSeats:%+v",
		t.banker, t.first, t.curBet, t.sitCnt, len(canGameSeats), chairs)
	t.mLog.begin(t.sitCnt, t.banker, t.first, t.curBet, chairs, canGameSeats)

}

// 检查准备用户
func (t *Table) checkStart() (bool, []*player.Player, []int32) {
	canGameSeats, chairs := []*player.Player(nil), []int32(nil)
	for _, v := range t.seats {
		if v == nil {
			continue
		}
		if v.GetAllMoney() < t.curBet {
			// 开局身上金币不够下注？
			continue
		}
		canGameSeats = append(canGameSeats, v)
		chairs = append(chairs, v.GetChairID())
	}
	return len(canGameSeats) >= 2, canGameSeats, chairs
}

// 扣钱 （或处理可以进行游戏的玩家状态等逻辑）
func (t *Table) intoGaming(canGameSeats []*player.Player) {
	for _, p := range canGameSeats {
		if !p.IntoGaming(t.curBet) {
			log.Errorf("intoGaming error. p:%+v currBet=%.1f", p.Desc(), t.curBet)
			continue
		}
		t.totalBet += t.curBet
	}
}

// 计算庄家位置
func (t *Table) calcBanker() {
	next := t.NextPlayer(t.banker)
	if next == nil {
		log.Errorf("calcBanker err. banker=%v", t.banker)
		return
	}
	t.curRound = 1
	t.banker = next.GetChairID()
	t.active = t.banker
	t.first = t.active
	t.broadcastSetBankerRsp()
}

// 发牌
func (t *Table) dispatchCard(canGameSeats []*player.Player) {
	// 洗牌
	t.cards.Shuffle()

	// 发牌
	for _, p := range canGameSeats {
		p.AddCards(t.cards.DispatchCards(3))
	}

	// 发牌广播
	t.dispatchCardPush(canGameSeats)
}

func (t *Table) onSendCardTimeout() {
	t.updateStage(StAction)
	t.broadcastActivePlayerPush()
}

func (t *Table) onActionTimeout() {
	t.OnActionReq(t.GetActivePlayer(), &v1.ActionReq{Action: AcPack}, true)
}

func (t *Table) onSideShowTimeout() {
	t.OnActionReq(t.GetActivePlayer(), &v1.ActionReq{Action: AcSideReply, SideReplyAllow: false}, true)
}

// 比牌赢家操作
func (t *Table) onSideShowAniTimeout() {
	if len(t.GetGamingPlayers()) <= 1 {
		return
	}
	t.updateStage(StAction)
	t.broadcastActivePlayerPush()
	t.checkRound(t.active)
}

/* 游戏结束 */
func (t *Table) gameEnd() {
	// 胜利的玩家
	var winner *player.Player
	for _, seat := range t.seats {
		if seat != nil && seat.IsGaming() {
			winner = seat
			break
		}
	}

	if winner == nil {
		t.updateStage(StEnd)
		log.Errorf("gameEnd err. winner=%+v", winner)
		return
	}

	// 结算
	profit := winner.Settle(t.totalBet)
	// t.Broadcast(-1, packet)
	// t.SendShowCard()
	t.broadcastResult()
	t.mLog.settle(profit)
	t.updateStage(StEnd)
}

func (t *Table) onEndTimeout() {
	// 游戏结束后判断
	t.checkKick()
	t.Reset()
	t.checkReady()
	log.Debugf("结束清理完成。\n")
	t.mLog.end(fmt.Sprintf("结束清理完成。"))
}
