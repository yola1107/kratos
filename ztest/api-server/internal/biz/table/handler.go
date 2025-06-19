package table

import (
	"github.com/yola1107/kratos/v2/library/ext"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
)

func (t *Table) OnExitGame(p *player.Player, code int32, msg string) bool {
	if !t.ThrowOff(p, false) {
		return false
	}
	t.repo.LogoutGame(p, code, msg)
	return false
}

func (t *Table) OnSceneReq(p *player.Player, isClient bool) {
	t.SendSceneInfo(p)
	return
}

func (t *Table) OnReadyReq(p *player.Player, isReady bool) bool {
	return true
}

func (t *Table) OnChatReq(p *player.Player, in *v1.ChatReq) bool {
	return true
}

func (t *Table) OnHosting(p *player.Player, isHosting bool) bool {
	return true
}

func (t *Table) OnAutoCallReq(p *player.Player, autoCall bool) bool {
	return true
}

func (t *Table) OnActionReq(p *player.Player, in *v1.ActionReq, isTimeOut bool) (ok bool) {
	if p == nil || !p.IsGaming() || len(t.GetCanActionPlayers()) <= 1 {
		return
	}

	stage := t.stage.state
	if stage == StWait || stage == StReady || stage == StWaitEnd || stage == StEnd {
		return
	}

	action := in.Action

	switch action {
	case PLAYER_SEE:
		t.handleSee(p)

	case PLAYER_PACK:
		t.handlePack(p, isTimeOut)

	case PLAYER_CALL, PLAYER_RAISE:
		t.handleCall(p, action)

	case PLAYER_SHOW:
		t.handleShow(p, action)

	case PLAYER_SIDE:
		t.handleSideShow(p, action)

	case PLAYER_SIDE_REPLY:
		t.handleSideShowReply(p, action, isTimeOut, false)
	}

	return true
}

func (t *Table) handleSee(p *player.Player) {
	if p.IsSee() {
		t.sendActionRsp(p, &v1.ActionRsp{Code: 1, Action: PLAYER_SEE})
		return
	}

	p.SetSee()
	t.broadcastActionRsp(p, PLAYER_SEE)

	if p.GetChairID() == t.active {
		// 刷新定时器，通知活动玩家
		t.active = p.GetChairID()
		t.updateStage(StAction)
		t.broadcastActivePlayerPush()
	} else {
		// 判断当前活动玩家是否可以发起比牌
		if canShow, _ := t.canShowCard(t.GetActivePlayer()); canShow {
			// 通知当前活动玩家添加比牌按钮
		}
	}
}

// 弃牌 允许非当前玩家操作
func (t *Table) handlePack(p *player.Player, isTimeout bool) {
	// 比牌阶段不可丢牌
	if p == nil || !p.IsGaming() || t.stage.state == StSideShow {
		return
	}

	p.IncrIdleCount(isTimeout)
	p.SetLastOp(PLAYER_PACK)
	p.SetStatus(player.StGameFold) // 弃牌标记
	t.broadcastActionRsp(p, PLAYER_PACK)

	if ps := t.GetCanActionPlayers(); len(ps) <= 1 {
		t.updateStage(StWaitEnd)
		return
	}

	if p.GetChairID() == t.active {
		// 通知下个玩家操作
		t.active = t.getNextActiveChair()
		t.updateStage(StAction)
		t.broadcastActivePlayerPush()
	}
}

func (t *Table) handleCall(p *player.Player, action int32) {
	if p.GetChairID() != t.active || t.stage.state != StAction || len(t.GetCanActionPlayers()) <= 2 {
		return
	}
	needMoney := t.calcBetMoney(p)
	if action == PLAYER_RAISE {
		needMoney *= 2
	}
	if !t.hasEnoughMoney(p, needMoney) {
		t.handlePack(p, false) // 直接弃牌处理
		return
	}

	t.totalBet += needMoney
	p.UseMoney(needMoney)
	p.AddBet(needMoney)
	p.SetLastOp(action)
	p.IncrPlayCount()
	t.broadcastActionRsp(p, action)

	// 判断是否需要处理所有比牌
	if t.totalBet >= t.repo.GetRoomConfig().Game.PotLimit {
		t.dealAllCompare()
		return
	}

	// 通知下个玩家操作
	t.active = t.getNextActiveChair()
	t.updateStage(StAction)
	t.broadcastActivePlayerPush()
}

func (t *Table) dealAllCompare() {
	t.updateStage(StWaitEnd)
}

func (t *Table) handleShow(p *player.Player, action int32) {
	if p.GetChairID() != t.active || t.stage.state != StAction || len(t.GetCanActionPlayers()) != 2 {
		return
	}
	needMoney := t.calcBetMoney(p)
	if !t.hasEnoughMoney(p, needMoney) {
		t.handlePack(p, false) // 直接弃牌处理
		return
	}

	t.totalBet += needMoney
	p.UseMoney(needMoney)
	p.AddBet(needMoney)
	p.SetLastOp(action)
	p.IncrPlayCount()

	// 比牌+展示结果

	// 等待结束
	t.updateStage(StWaitEnd)
}

func (t *Table) handleSideShow(p *player.Player, action int32) {
	if p.GetChairID() != t.active || t.stage.state != StAction || len(t.GetCanActionPlayers()) <= 2 {
		return
	}
	last := t.LastPlayer(p.GetChairID())
	if last == nil {
		return
	}
	if !last.IsSee() || !p.IsSee() {
		return
	}
	needMoney := t.calcBetMoney(p)
	if !t.hasEnoughMoney(p, needMoney) {
		t.handlePack(p, false) // 直接弃牌处理
		return
	}

	t.totalBet += needMoney
	p.UseMoney(needMoney)
	p.AddBet(needMoney)
	p.SetLastOp(action)
	p.IncrPlayCount()
	t.broadcastActionRsp(p, action)

	// 判断是否需要处理所有比牌
	if t.totalBet >= t.repo.GetRoomConfig().Game.PotLimit {
		t.dealAllCompare()
		return
	}

	// 向上家发起提前比牌请求

	// 设置比牌的玩家椅子
	p.SetCompareSeats([]int32{last.GetChairID()})

	// 等待上家提前比牌回应
	t.active = last.GetChairID()
	t.updateStage(StSideShow)
	t.broadcastActivePlayerPush()
}

func (t *Table) handleSideShowReply(p *player.Player, action int32, isTimeout bool, allow bool) {
	if p.GetChairID() != t.active || t.stage.state != StSideShow || len(t.GetCanActionPlayers()) <= 2 {
		return
	}

	next := t.NextPlayer(p.GetChairID())
	if next == nil || !next.IsGaming() || !next.IsSee() || !p.IsSee() {
		return
	}
	if !ext.SliceContains(next.GetCompareSeats(), p.GetChairID()) {
		return
	}

	// 拒绝比牌
	if !allow {
		// 通知发起比牌的玩家操作
		t.active = next.GetChairID()
		t.updateStage(StAction)
		t.broadcastActivePlayerPush()
		return
	}

	// 同意比牌
	winner, loss := getWinner(p, next)

	// 设置输家的状态
	loss.SetStatus(player.StGameLost) // 输家标记
	loss.SetLastOp(action)            //
	winner.SetLastOp(action)          //

	// 通知赢家操作
	t.active = winner.GetChairID()
	t.updateStage(StAction)
	t.broadcastActivePlayerPush()
}

func getWinner(p1, p2 *player.Player) (winner, loss *player.Player) {
	// compare
	if p1.GetCardsType() > p2.GetCardsType() {
		return p1, p2
	}
	return p2, p1
}

// calcBetMoney 计算当前玩家根据是否看牌应支付的金额
func (t *Table) calcBetMoney(p *player.Player) float64 {
	if p.IsSee() {
		return t.curBet * 2
	}
	return t.curBet
}

// hasEnoughMoney 检查玩家是否有足够的钱下注
func (t *Table) hasEnoughMoney(p *player.Player, amount float64) bool {
	return p.GetAllMoney() >= amount
}

func (t *Table) notifyAction() {

}
