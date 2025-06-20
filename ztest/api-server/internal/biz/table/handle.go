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

/*


 */

const (
	ErrOK int32 = iota
	ErrInvalidStage
	ErrNotEnoughMoney
	ErrorBeSeen
	ErrNotSeen
	ErrTargetInvalid
)

type ActionRet struct { // 检查结果
	Code    int32
	Money   float64
	Target  *player.Player
	Message string // 可选，用于调试或客户端提示
}

func (t *Table) OnActionReq(p *player.Player, in *v1.ActionReq, isTimeOut bool) (ok bool) {
	if p == nil || !p.IsGaming() || len(t.GetCanActionPlayers()) <= 1 {
		return
	}

	stage := t.stage.state
	if stage == StWait || stage == StReady || stage == StWaitEnd || stage == StEnd {
		return
	}

	switch action := in.Action; action {
	case AcSee:
		t.handleSee(p)
	case AcPack:
		t.handlePack(p, isTimeOut)
	case AcCall, AcRaise:
		t.handleCall(p, action)
	case AcShow:
		t.handleShow(p, action)
	case AcSide:
		t.handleSideShow(p, action)
	case AcSideReply:
		t.handleSideShowReply(p, action, isTimeOut, false)
	}
	return true
}

// see
func (t *Table) canSeeCard(p *player.Player) ActionRet {
	if p == nil || p.IsSee() {
		return ActionRet{Code: ErrorBeSeen}
	}
	if t.stage.state != StSendCard && t.stage.state != StAction && t.stage.state != StSideShow {
		return ActionRet{Code: ErrInvalidStage}
	}
	return ActionRet{}
}

func (t *Table) handleSee(p *player.Player) {
	if ret := t.canSeeCard(p); ret.Code != ErrOK {
		t.sendActionRsp(p, &v1.ActionRsp{Code: ret.Code, Action: AcSee})
		return
	}

	p.SetSee()
	t.broadcastActionRsp(p, AcSee, 0, nil, false)

	if p.GetChairID() == t.active {
		// 刷新定时器，通知活动玩家
		t.active = p.GetChairID()
		t.updateStage(StAction)
		t.broadcastActivePlayerPush()
	} else {
		// 判断当前活动玩家是否可以发起比牌
		t.sendActiveButtonInfoNtf()
	}
}

// 弃牌 允许非当前玩家操作
func (t *Table) canPack(p *player.Player) ActionRet {
	// 比牌阶段不可丢牌
	if p == nil || !p.IsGaming() || t.stage.state == StSideShow {
		return ActionRet{Code: ErrInvalidStage}
	}
	return ActionRet{}
}

func (t *Table) handlePack(p *player.Player, isTimeout bool) {
	if ret := t.canPack(p); ret.Code != ErrOK {
		return
	}

	p.IncrIdleCount(isTimeout)
	p.SetLastOp(AcPack)
	p.SetStatus(player.StGameFold) // 弃牌标记
	t.broadcastActionRsp(p, AcPack, 0, nil, false)

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

// 跟注（Call） 加注（Raise）
func (t *Table) canCallCard(p *player.Player, isRaise bool) (callRes ActionRet) {
	if p == nil || p.GetChairID() != t.active || t.stage.state != StAction || len(t.GetCanActionPlayers()) <= 2 {
		return ActionRet{Code: ErrInvalidStage}
	}
	callMoney := t.calcBetMoney(p)
	if isRaise {
		callMoney *= 2
	}
	if !t.hasEnoughMoney(p, callMoney) {
		return ActionRet{Code: ErrNotEnoughMoney}
	}
	return ActionRet{Code: ErrOK, Money: callMoney}
}

func (t *Table) handleCall(p *player.Player, action int32) {
	callRet := t.canCallCard(p, action == AcRaise)
	if callRet.Code != ErrOK {
		if callRet.Code == ErrNotEnoughMoney {
			t.handlePack(p, false) // 直接弃牌处理
		}
		return
	}

	needMoney := callRet.Money
	t.totalBet += needMoney
	p.UseMoney(needMoney)
	p.AddBet(needMoney)
	p.SetLastOp(action)
	p.IncrPlayCount()
	t.broadcastActionRsp(p, action, needMoney, nil, false)

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

// Show
// 当只剩 2 名玩家时，任意一方可请求 Show
// 明牌比较三张牌，胜者赢取全部筹码
func (t *Table) canShowCard(p *player.Player) ActionRet {
	if p == nil || p.GetChairID() != t.active || t.stage.state != StAction || len(t.GetCanActionPlayers()) != 2 {
		return ActionRet{Code: ErrInvalidStage}
	}
	next := t.NextPlayer(p.GetChairID())
	if next == nil {
		return ActionRet{Code: ErrTargetInvalid}
	}
	money := t.calcBetMoney(p)
	if !t.hasEnoughMoney(p, money) {
		return ActionRet{Code: ErrNotEnoughMoney}
	}
	return ActionRet{Code: ErrOK, Money: money, Target: next}
}

func (t *Table) handleShow(p *player.Player, action int32) {
	ret := t.canShowCard(p)
	if ret.Code != ErrOK {
		if ret.Code == ErrNotEnoughMoney {
			t.handlePack(p, false) // 直接弃牌处理
		}
		return
	}

	needMoney := ret.Money
	t.totalBet += needMoney
	p.UseMoney(needMoney)
	p.AddBet(needMoney)
	p.SetLastOp(action)
	p.IncrPlayCount()
	t.broadcastActionRsp(p, action, needMoney, ret.Target, false)

	// 比牌+展示结果
	winner, loss := getWinner(p, ret.Target)

	// 设置输家的状态
	loss.SetStatus(player.StGameLost) // 输家标记
	loss.SetLastOp(action)            //
	winner.SetLastOp(action)          //

	// 等待结束
	t.updateStage(StWaitEnd)
}

// Side Show
// 剩余玩家数量 > 2
// 仅限明注玩家对上一位明注玩家请求比牌
// 若对方同意，则比大小，小的一方自动弃牌
func (t *Table) canSideShowCard(p *player.Player) ActionRet {
	if p == nil || p.GetChairID() != t.active || t.stage.state != StAction || len(t.GetCanActionPlayers()) <= 2 {
		return ActionRet{Code: ErrInvalidStage}
	}
	last := t.LastPlayer(p.GetChairID())
	if last == nil || last == p {
		return ActionRet{Code: ErrTargetInvalid}
	}
	if !last.IsSee() || !p.IsSee() {
		return ActionRet{Code: ErrNotSeen}
	}
	needMoney := t.calcBetMoney(p)
	if !t.hasEnoughMoney(p, needMoney) {
		return ActionRet{Code: ErrNotEnoughMoney}
	}
	return ActionRet{Code: ErrOK, Money: needMoney, Target: last}
}

func (t *Table) handleSideShow(p *player.Player, action int32) {
	ret := t.canSideShowCard(p)
	if ret.Code != ErrOK {
		if ret.Code == ErrNotEnoughMoney {
			t.handlePack(p, false) // 直接弃牌处理
		}
		return
	}
	needMoney := ret.Money
	last := ret.Target

	t.totalBet += needMoney
	p.UseMoney(needMoney)
	p.AddBet(needMoney)
	p.SetLastOp(action)
	p.IncrPlayCount()
	t.broadcastActionRsp(p, action, needMoney, last, false)

	// 判断是否需要处理所有比牌
	if t.totalBet >= t.repo.GetRoomConfig().Game.PotLimit {
		t.dealAllCompare()
		return
	}

	// 设置比牌的玩家椅子
	p.SetCompareSeats([]int32{last.GetChairID()})

	// 等待上家提前比牌回应
	t.active = last.GetChairID()
	t.updateStage(StSideShow)
	t.broadcastActivePlayerPush()
}

// Side Show Reply
// 能否回应提前比牌
func (t *Table) canSideShowReply(p *player.Player) ActionRet {
	if p == nil || p.GetChairID() != t.active || t.stage.state != StSideShow || len(t.GetCanActionPlayers()) <= 2 {
		return ActionRet{Code: ErrInvalidStage}
	}
	next := t.NextPlayer(p.GetChairID())
	if next == nil || !next.IsGaming() || !next.IsSee() {
		return ActionRet{Code: ErrTargetInvalid}
	}
	if !ext.SliceContains(next.GetCompareSeats(), p.GetChairID()) {
		return ActionRet{Code: ErrTargetInvalid}
	}
	return ActionRet{Code: ErrOK, Target: next}
}

// 回应提前比牌
func (t *Table) handleSideShowReply(p *player.Player, action int32, isTimeout bool, allow bool) {
	ret := t.canSideShowReply(p)
	if ret.Code != ErrOK {
		return
	}
	next := ret.Target

	// 广播结果
	t.broadcastActionRsp(p, action, 0, next, allow)

	// 拒绝比牌
	if !allow {
		// 通知发起比牌玩家的下家操作
		t.active = t.NextPlayer(next.GetChairID()).GetChairID()
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
	t.updateStage(StSideShowAni)
	t.broadcastActivePlayerPush()
}

// 处理所有玩家比牌
func (t *Table) dealAllCompare() {
	t.updateStage(StWaitEnd)
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

func (t *Table) checkAutoSee() {
	if t.curRound >= t.repo.GetRoomConfig().Game.AutoSeeRound {
		t.RangePlayer(func(k int32, p *player.Player) bool {
			if p.IsGaming() {
				t.OnActionReq(p, &v1.ActionReq{Action: AcSee}, false)
			}
			return true
		})
	}
}
