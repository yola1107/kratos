package table

import (
	"fmt"
	"time"

	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/player"
)

/*
	游戏主逻辑
*/

// MinStartPlayerCnt 最小开局人数
const MinStartPlayerCnt = 2

func (t *Table) OnTimer() {
	state := t.stage.GetState()
	// timerID := t.stage.GetTimerID()
	// log.Debugf("[Stage] OnTimer timeout. St:%v TimerID=%d", state, timerID)

	switch state {
	case StWait:
		// log.Debugf("StWait timeout. tb:%v ", t.Desc())
	case StReady:
		t.onGameStart()
	case StSendCard:
		t.onSendCardTimeout()
	case StPlaying:
		t.onActionTimeout()
	case StWaitEnd:
		t.gameEnd()
	case StEnd:
		t.onEndTimeout()
	default:
		log.Errorf("unhandled stage timeout: %v ", state)
	}
}

func (t *Table) updateStage(state StageID) {
	timeout := time.Duration(state.Timeout()) * time.Second
	t.updateStageWith(state, timeout)
}

func (t *Table) updateStageWith(state StageID, duration time.Duration) {
	// 获取当前定时器ID并取消
	currentTimerID := t.stage.GetTimerID()
	t.repo.GetTimer().Cancel(currentTimerID)

	// 启动新定时器
	timerID := t.repo.GetTimer().Once(duration, t.OnTimer)

	// 更新阶段
	t.stage.Set(state, duration, timerID)

	// 日志
	t.mLog.stage(t.stage.Desc(), t.active)
	// log.Debugf("[Stage] ====> %v, timerID: %d->%d", t.stage.Desc(), currentTimerID, timerID)
}

func (t *Table) checkCanStart() {
	if t.stage.GetState() != StWait {
		return
	}

	if canStart := t.checkReadyPlayer(); !canStart {
		return
	}

	// 准备开局
	t.updateStage(StReady)
}

func (t *Table) onGameStart() {
	// 再次检查是否可进行游戏; 兜底回退到StWait
	can, seats, infos := t.checkReadyInfos()
	if !can || t.stage.GetState() != StReady {
		t.updateStage(StWait)
		return
	}

	// 扣钱
	t.intoGaming(seats)

	// 计算庄家及操作玩家
	t.calcBanker(seats)

	// 发牌
	t.dispatchCard(seats)

	// 发牌状态倒计时3s
	t.updateStage(StSendCard)

	log.Debugf("******** <游戏开始> %s GamerInfo=%+v all=%v", t.Desc(), infos, logPlayers(t.seats))
	t.mLog.begin(t.Desc(), t.repo.GetRoomConfig().Game.BaseMoney, t.seats, infos)
}

// 检查用户是否可以开局
func (t *Table) checkReadyPlayer() bool {
	okCnt := 0
	for _, v := range t.seats {
		if v == nil || !v.IsReady() || v.GetAllMoney() < t.repo.GetRoomConfig().Game.BaseMoney {
			continue
		}
		okCnt++
	}
	return okCnt >= MinStartPlayerCnt
}

func (t *Table) checkReadyInfos() (bool, []*player.Player, []string) {
	canGameInfo := []string(nil)
	canGameSeats := []*player.Player(nil)
	for _, v := range t.seats {
		if v == nil || v.GetAllMoney() < t.repo.GetRoomConfig().Game.BaseMoney || !v.IsReady() {
			continue
		}
		canGameSeats = append(canGameSeats, v)
		canGameInfo = append(canGameInfo, fmt.Sprintf("%d:%d", v.GetPlayerID(), v.GetChairID()))
	}

	return len(canGameSeats) >= MinStartPlayerCnt, canGameSeats, canGameInfo
}

// 检查是否自动准备
func (t *Table) checkAutoReady(p *player.Player) {
	if !t.repo.GetRoomConfig().Game.AutoReady {
		return
	}
	if p != nil && !p.IsReady() && p.GetAllMoney() >= t.repo.GetRoomConfig().Game.BaseMoney {
		p.SetReady()
	}
}

func (t *Table) checkAutoReadyAll() {
	if !t.repo.GetRoomConfig().Game.AutoReady {
		return
	}
	for _, p := range t.seats {
		if p != nil && !p.IsReady() && p.GetAllMoney() >= t.repo.GetRoomConfig().Game.BaseMoney {
			p.SetReady()
		}
	}
}

// 扣钱 （或处理可以进行游戏的玩家状态等逻辑）
func (t *Table) intoGaming(seats []*player.Player) {
	for _, p := range seats {
		if p == nil {
			continue
		}
		p.SetGaming()
		if !p.IntoGaming(t.repo.GetRoomConfig().Game.BaseMoney) {
			log.Errorf("intoGaming error. p:%+v currBet=%.1f", p.Desc(), t.repo.GetRoomConfig().Game.BaseMoney)
		}
	}
}

// 计算庄家/首家位置
func (t *Table) calcBanker(seats []*player.Player) {
	idx := ext.RandInt(0, len(seats))
	t.active = seats[int32(idx)].GetChairID()
	t.first = t.active
}

// 发牌
func (t *Table) dispatchCard(seats []*player.Player) {
	// 洗牌
	t.cards.Shuffle()

	// 发牌
	for _, p := range seats {
		p.AddCards(t.cards.DispatchCards(5))
	}

	// 设置底牌
	bottom := t.cards.SetBottom()
	leftNum := t.cards.GetCardNum()

	// 设置桌面操作的牌
	t.currCard = bottom[0]

	// 发牌广播
	t.dispatchCardPush(seats, bottom, leftNum)
}

func (t *Table) onSendCardTimeout() {
	t.updateStage(StPlaying)
	t.broadcastActivePlayerPush()
}

func (t *Table) onActionTimeout() {

}

/* 游戏结束 */
func (t *Table) gameEnd() {

	t.settle()

	t.broadcastResult()

	// 保存每一局记录 // todo

	// 重置玩家状态
	t.setSitStatus()

	// 检查踢人
	t.checkKick()

	// 清理数据
	t.Reset()

	log.Debugf("结束清理完成。tb=%v \n", t.Desc())
	t.mLog.end(fmt.Sprintf("结束清理完成。%s %s", t.Desc(), logPlayers(t.seats)))

	// 状态转移
	t.updateStage(StEnd)
}

func (t *Table) settle() {
	// 胜利的玩家
	var winner *player.Player
	for _, seat := range t.seats {
		if seat != nil && seat.IsGaming() {
			winner = seat
			break
		}
	}

	if winner == nil {
		log.Errorf("settle winner == nil. tb=%+v", t.Desc())
		return
	}

	// 结算
	// winner.Settle(t.totalBet)

	t.mLog.settle(winner)
	// log.Debugf("gameEnd tb=%s winner=%+v", t.Desc(), winner.Desc())
}

func (t *Table) setSitStatus() {
	for _, v := range t.seats {
		if v == nil {
			continue
		}
		v.SetSit()
	}
}

func (t *Table) onEndTimeout() {
	// 状态进入 StWait
	t.updateStage(StWait)

	// 再次检查踢人
	t.checkKick()

	// 是否自动准备
	t.checkAutoReadyAll()

	// 是否可以下一局
	t.checkCanStart()
}
