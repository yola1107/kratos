package table

import (
	"time"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
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
	log.Infof("Stage=%d timeID=%d TimeOut... ", t.stage.state, t.stage.timerID)

	switch t.stage.state {
	case conf.StReady:
		t.onGameStart()
	case conf.StSendCard:
		t.onSendCardTimeout()
		// t.notifyAction(false, ACTION)
	case conf.StAction: // 超时操作
		t.onActionTimeout()
		// t.OnAction(t.CurrPlayer(), network.Packet{"action": PLAYER_PACK}, true)
	case conf.StWaitSiderShow: // 比牌操作超时
		// t.OnAction(t.CurrPlayer(), network.Packet{"action": PLAYER_OK_SIDER_SHOW, "allow": false}, true)
	case conf.StSiderShow: // 操作之后等待时间
		// t.notifyAction(true, ACTION)
	case conf.StWaitEnd:
		// t.gameEnd()
	case conf.StEnd: // 游戏结束后判断
		// t.clearAnomalyPlayers()
		// t.Reset()
		// t.checkReady()
		// t.mLog.End(fmt.Sprintf("结束清理完成。"))
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
	log.Infof("stage changed. timerID(%d) stage:(%d -> %d) ", t.stage.timerID, t.stage.prev, t.stage.state)
}

func (t *Table) checkResetDuration(s int32) time.Duration {
	timeout := conf.GetStageTimeout(s)
	// 检查是否调整超时时间
	return time.Duration(timeout) * time.Second
}

func (t *Table) checkReady() {
	okCnt := int16(0)
	t.RangePlayer(func(k int32, p *player.Player) bool {
		if p.IsReady() && p.GetMoney() >= t.curBet {
			okCnt++
		}
		return true
	})
	canStart := okCnt >= 2
	if !canStart {
		t.stage.state = conf.StWait
		return
	}

	// 准备状态倒计时2s
	t.updateStage(conf.StReady)
}

func (t *Table) onGameStart() {
	can, canGameSeats, chairs := t.checkStart()
	if !can {
		t.stage.state = conf.StWait
		return
	}

	log.Debugf("******** <游戏开始> 当前局:%d sitCnt=%d canGameSeats:%+v",
		t.curRound, t.sitCnt, chairs)

	// 扣钱
	t.intoGaming(canGameSeats)

	// 计算庄家及操作玩家
	t.calcBanker()

	// 发牌
	t.dispatchCard(canGameSeats)

	// 发牌状态倒计时3s
	t.updateStage(conf.StSendCard)
}

// 检查准备用户
func (t *Table) checkStart() (bool, []*player.Player, []int32) {
	canGameSeats, chairs := []*player.Player(nil), []int32(nil)
	for _, v := range t.seats {
		if v == nil {
			continue
		}
		if v.GetMoney() < t.curBet {
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
	t.banker = next.GetChairID()
	t.active = t.banker
	t.curRound = 1
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
	t.updateStage(conf.StAction)
	t.broadcastActivePlayerPush()
}

func (t *Table) onActionTimeout() {

}
