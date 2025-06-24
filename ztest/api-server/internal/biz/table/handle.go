package table

import (
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/calc"
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

func (t *Table) OnActionReq(p *player.Player, in *v1.ActionReq, timeout bool) (ok bool) {
	if p == nil || !p.IsGaming() || len(t.GetGamers()) <= 1 {
		return
	}

	stage := t.stage.State
	if stage == StWait || stage == StReady || stage == StWaitEnd || stage == StEnd {
		return
	}

	if timeout && p.IsRobot() || in.Action == -1 {
		log.Errorf("OnActionReq err. p:%v ac=%v CanOp=%v gamer=%+v ",
			p.Desc(), in.Action, t.getCanOp(p), len(t.GetGamers()))
	}
	log.Debugf("=> p:%v ac=%v CanOp=%v gamer=%+v timeout=%v ",
		p.Desc(), in.Action, t.getCanOp(p), len(t.GetGamers()), timeout)

	switch in.Action {
	case v1.ACTION_SEE:
		t.handleSee(p, timeout)
	case v1.ACTION_PACK:
		t.handlePack(p, in, timeout)
	case v1.ACTION_CALL, v1.ACTION_RAISE:
		t.handleCall(p, in, timeout)
	case v1.ACTION_SHOW:
		t.handleShow(p, in, timeout)
	case v1.ACTION_SIDE:
		t.handleSideShow(p, in, timeout)
	case v1.ACTION_SIDE_REPLY:
		t.handleSideShowReply(p, in, timeout)
	}
	return true
}

// see
func (t *Table) canSeeCard(p *player.Player) ActionRet {
	if p == nil || p.Seen() {
		return ActionRet{Code: ErrorAlreadySeen}
	}
	if t.stage.State != StSendCard && t.stage.State != StAction && t.stage.State != StSideShow {
		return ActionRet{Code: ErrInvalidStage}
	}
	return ActionRet{}
}

func (t *Table) handleSee(p *player.Player, timeout bool) {
	if ret := t.canSeeCard(p); ret.Code != ErrOK {
		t.sendActionRsp(p, &v1.ActionRsp{Code: ret.Code, Action: v1.ACTION_SEE})
		return
	}

	p.SetSeen()
	p.SetLastOp(v1.ACTION_SEE)
	p.IncrTimeoutCnt(timeout)
	t.broadcastActionRsp(p, v1.ACTION_SEE, 0, nil, false)
	t.mLog.SeeCard(p)

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
	if p == nil || !p.IsGaming() || t.stage.State == StSideShow {
		return ActionRet{Code: ErrInvalidStage}
	}
	return ActionRet{}
}

func (t *Table) handlePack(p *player.Player, in *v1.ActionReq, timeout bool) {
	if ret := t.canPack(p); ret.Code != ErrOK {
		t.sendActionRsp(p, &v1.ActionRsp{Code: ret.Code, Action: v1.ACTION_PACK})
		return
	}

	p.SetFold() // 弃牌标记
	t.addBetInfo(p, in.Action, timeout, 0)
	t.broadcastActionRsp(p, in.Action, 0, nil, false)
	t.mLog.PackCard(p, timeout)

	if ps := t.GetGamers(); len(ps) <= 1 {
		t.updateStage(StWaitEnd)
		return
	}

	next := t.getNextActiveChair()
	if p.GetChairID() == t.first {
		t.first = next
	}
	if p.GetChairID() == t.active {
		// 通知下个玩家操作
		t.active = next
		t.checkRound(t.active)
		t.updateStage(StAction)
		t.broadcastActivePlayerPush()
	}
}

// 跟注（Call） 加注（Raise）
func (t *Table) canCallCard(p *player.Player, isRaise bool) (callRes ActionRet) {
	if p == nil || p.GetChairID() != t.active || t.stage.State != StAction {
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

func (t *Table) handleCall(p *player.Player, in *v1.ActionReq, timeout bool) {
	callRaise := in.Action == v1.ACTION_RAISE
	ret := t.canCallCard(p, callRaise)
	if ret.Code != ErrOK {
		if ret.Code == ErrNotEnoughMoney {
			t.OnActionReq(p, &v1.ActionReq{Action: v1.ACTION_PACK}, false) // 直接弃牌处理
		}
		return
	}
	needMoney := ret.Money

	t.addBetInfo(p, in.Action, timeout, needMoney)
	t.broadcastActionRsp(p, in.Action, needMoney, nil, false)
	t.mLog.CallCard(p, needMoney, callRaise, timeout)

	// 判断是否需要处理所有比牌
	if t.totalBet >= t.repo.GetRoomConfig().Game.PotLimit {
		t.dealCompare(t.GetGamers(), CompareAllShow) // 处理所有比牌
		return
	}

	// 通知下个玩家操作
	t.active = t.getNextActiveChair()
	t.checkRound(t.active)
	t.updateStage(StAction)
	t.broadcastActivePlayerPush()
}

// Show
// 当只剩 2 名玩家时，任意一方可请求 Show
// 明牌比较三张牌，胜者赢取全部筹码
func (t *Table) canShowCard(p *player.Player) ActionRet {
	if p == nil || p.GetChairID() != t.active || t.stage.State != StAction || len(t.GetGamers()) != 2 {
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

func (t *Table) handleShow(p *player.Player, in *v1.ActionReq, timeout bool) {
	ret := t.canShowCard(p)
	if ret.Code != ErrOK {
		if ret.Code == ErrNotEnoughMoney {
			t.OnActionReq(p, &v1.ActionReq{Action: v1.ACTION_PACK}, false) // 直接弃牌处理
		}
		return
	}
	needMoney := ret.Money

	t.addBetInfo(p, in.Action, timeout, needMoney)
	t.broadcastActionRsp(p, in.Action, needMoney, ret.Target, false)
	t.mLog.ShowCard(p, ret.Target, needMoney, timeout)
	t.dealCompare(t.GetGamers(), CompareShow) // 处理比牌 2个玩家
}

// Side Show
// 剩余玩家数量 > 2
// 仅限明注玩家对上一位明注玩家请求比牌
// 若对方同意，则比大小，小的一方自动弃牌
func (t *Table) canSideShowCard(p *player.Player) ActionRet {
	if p == nil || p.GetChairID() != t.active || t.stage.State != StAction || len(t.GetGamers()) <= 2 {
		return ActionRet{Code: ErrInvalidStage}
	}
	last := t.LastPlayer(p.GetChairID())
	if last == nil || last == p {
		return ActionRet{Code: ErrTargetInvalid}
	}
	if !last.Seen() || !p.Seen() {
		return ActionRet{Code: ErrSideNotSeen}
	}
	needMoney := t.calcBetMoney(p)
	if !t.hasEnoughMoney(p, needMoney) {
		return ActionRet{Code: ErrNotEnoughMoney}
	}
	return ActionRet{Code: ErrOK, Money: needMoney, Target: last}
}

func (t *Table) handleSideShow(p *player.Player, in *v1.ActionReq, timeout bool) {
	ret := t.canSideShowCard(p)
	if ret.Code != ErrOK {
		if ret.Code == ErrNotEnoughMoney {
			t.OnActionReq(p, &v1.ActionReq{Action: v1.ACTION_PACK}, false) // 直接弃牌处理
		}
		return
	}
	needMoney := ret.Money
	last := ret.Target

	t.addBetInfo(p, in.Action, timeout, needMoney)
	t.broadcastActionRsp(p, in.Action, needMoney, last, false)
	t.mLog.SidedShow(p, ret.Target, needMoney, timeout)
	// log.Debugf("sideshow lunch. from:%v to:%v timeout:%v", p.Desc(), last.Desc(), timeout)

	// 判断是否需要处理所有比牌
	if t.totalBet >= t.repo.GetRoomConfig().Game.PotLimit {
		t.dealCompare(t.GetGamers(), CompareAllShow) // 处理所有比牌
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
	if p == nil || p.GetChairID() != t.active || t.stage.State != StSideShow || len(t.GetGamers()) <= 2 {
		return ActionRet{Code: ErrInvalidStage}
	}
	next := t.NextPlayer(p.GetChairID())
	if next == nil || !next.IsGaming() || !next.Seen() {
		return ActionRet{Code: ErrTargetInvalid}
	}
	if !ext.SliceContains(next.GetCompareSeats(), p.GetChairID()) {
		return ActionRet{Code: ErrTargetInvalid}
	}
	return ActionRet{Code: ErrOK, Target: next}
}

// 回应提前比牌
func (t *Table) handleSideShowReply(p *player.Player, in *v1.ActionReq, timeout bool) {
	ret := t.canSideShowReply(p)
	if ret.Code != ErrOK {
		return
	}
	next := ret.Target

	p.SetLastOp(in.Action)
	p.IncrTimeoutCnt(timeout)
	t.broadcastActionRsp(p, in.Action, 0, next, in.SideReplyAllow)
	t.mLog.SideShowReply(p, ret.Target, in.SideReplyAllow, timeout)
	// log.Debugf("sideshow Reply. from:%v tar:%v timeout:%v", p.Desc(), next.Desc(), timeout)

	// 拒绝比牌
	if !in.SideReplyAllow {
		// 通知发起比牌玩家的下家操作
		t.active = t.NextPlayer(next.GetChairID()).GetChairID()
		t.checkRound(t.active)
		t.updateStage(StAction)
		t.broadcastActivePlayerPush()
		return
	}

	// 同意比牌
	winner, losses := t.dealCompare([]*player.Player{p, next}, CompareSideShow)
	loss := losses[0]
	if loss.GetChairID() == t.first {
		t.first = t.NextPlayer(loss.GetChairID()).GetChairID()
	}

	// 通知赢家操作
	t.active = winner.GetChairID()
	t.updateStage(StSideShowAni)
}

func (t *Table) addBetInfo(p *player.Player, action v1.ACTION, timeout bool, bet float64) {
	switch action {
	// 统计下注额,超时次数
	case v1.ACTION_PACK, v1.ACTION_CALL, v1.ACTION_RAISE, v1.ACTION_SHOW, v1.ACTION_SIDE:
		p.UseMoney(bet)
		p.AddBet(bet)
		p.IncrPlayCnt()
		p.SetLastOp(action)
		p.IncrTimeoutCnt(timeout)
		t.totalBet += bet // 累加
	}

	// 更新桌面下注额
	if action == v1.ACTION_RAISE {
		t.curBet *= 2
	}
}

func (t *Table) checkRound(active int32) {
	// 是否更新回合
	if active == t.first {
		t.curRound++
	}

	// 自动操作看
	if t.curRound >= t.repo.GetRoomConfig().Game.AutoSeeRound {
		t.RangePlayer(func(k int32, p *player.Player) bool {
			if p.IsGaming() {
				t.OnActionReq(p, &v1.ActionReq{Action: v1.ACTION_SEE}, false)
			}
			return true
		})
	}
}

// 处理玩家比牌
func (t *Table) dealCompare(compares []*player.Player, kind CompareType) (winner *player.Player, loss []*player.Player) {
	if len(compares) <= 1 {
		return compares[0], compares
	}

	winner = compares[0]
	for i, v := range compares {
		if i == 0 {
			winner = v
			continue
		}
		w, l := compareCard(winner, v)
		winner = w
		loss = append(loss, l)
		l.SetLost() // 比牌输家标记
	}

	t.mLog.compareCard(kind, winner, loss)
	log.Debugf("[玩家比牌] %s", logCompare(kind, winner, loss))
	if len(t.GetGamers()) <= 1 {
		t.updateStage(StWaitEnd) // 等待结束
	}
	return
}

func compareCard(p1, p2 *player.Player) (winner, loss *player.Player) {
	if calc.Compare(p1.GetHand(), p2.GetHand()) {
		return p1, p2
	}
	return p2, p1
}

// calcBetMoney 计算当前玩家根据是否看牌应支付的金额
func (t *Table) calcBetMoney(p *player.Player) float64 {
	if p.Seen() {
		return t.curBet * 2
	}
	return t.curBet
}

// hasEnoughMoney 检查玩家是否有足够的钱下注
func (t *Table) hasEnoughMoney(p *player.Player, amount float64) bool {
	return p.GetAllMoney() >= amount
}
