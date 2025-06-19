package table

import (
	"github.com/golang/protobuf/proto"
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
)

func (t *Table) SendPacketToClient(p *player.Player, cmd v1.GameCommand, msg proto.Message) {
	if p == nil {
		return
	}
	if p.IsRobot() {
		// tb.GetRbLogic().RecvMsg(p, cmd, msg)
		return
	}
	session := p.GetSession()
	if session == nil {
		return
	}
	if err := session.Push(int32(cmd), msg); err != nil {
		log.Warnf("send packet to client error: %v", err)
	}
}

func (t *Table) SendPacketToAll(cmd v1.GameCommand, msg proto.Message) {
	for _, v := range t.seats {
		if v == nil {
			continue
		}
		t.SendPacketToClient(v, cmd, msg)
	}
}

func (t *Table) SendPacketToAllExcept(cmd v1.GameCommand, msg proto.Message, uids ...int64) {
	exceptMap := make(map[int64]struct{})
	for _, v := range uids {
		exceptMap[v] = struct{}{}
	}
	for _, v := range t.seats {
		if v == nil {
			continue
		}
		if _, ok := exceptMap[v.GetPlayerID()]; ok {
			continue
		}
		t.SendPacketToClient(v, cmd, msg)
	}
}

// SendLoginRsp 发送玩家登录信息
func (t *Table) SendLoginRsp(p *player.Player, code int32, msg string) {
	t.SendPacketToClient(p, v1.GameCommand_OnLoginRsp, &v1.LoginRsp{
		Code:    code,
		Msg:     msg,
		UserID:  p.GetPlayerID(),
		TableID: p.GetTableID(),
		ChairID: p.GetChairID(),
		ArenaID: int32(conf.ArenaID),
	})
}

// 广播入座信息
func (t *Table) broadcastUserInfo(p *player.Player) {
	t.sendUserInfoToAnother(p, p)
	for k, v := range t.seats {
		if v != nil && k != int(p.GetChairID()) {
			t.sendUserInfoToAnother(p, v)
			t.sendUserInfoToAnother(v, p)
		}
	}
}

func (t *Table) sendUserInfoToAnother(src *player.Player, dst *player.Player) {
	t.SendPacketToClient(dst, v1.GameCommand_OnUserInfoPush, &v1.UserInfoPush{
		UserID:    src.GetPlayerID(),
		ChairId:   src.GetChairID(),
		UserName:  src.GetNickName(),
		Money:     src.GetMoney(),
		Avatar:    src.GetAvatar(),
		AvatarUrl: src.GetAvatarUrl(),
		Vip:       src.GetVipGrade(),
		Status:    int32(src.GetStatus()),
		Ip:        src.GetIP(),
	})
}

// SendSceneInfo 发送游戏场景信息
func (t *Table) SendSceneInfo(p *player.Player) {
	c := t.repo.GetRoomConfig()
	rsp := &v1.SceneRsp{
		BaseScore:    c.Game.BaseMoney,
		ChLimit:      c.Game.ChLimit,
		PotLimit:     c.Game.PotLimit,
		AutoSeeRound: c.Game.AutoSeeRound,
		Stage:        t.stage.state,
		Timeout:      int64(t.calcRemainingTime().Seconds()),
		Active:       t.active,
		Banker:       t.banker,
		CurRound:     t.curRound,
		TotalBet:     t.totalBet,
		Players:      t.getPlayersScene(),
	}
	t.SendPacketToClient(p, v1.GameCommand_OnSceneRsp, rsp)
}

func (t *Table) getPlayersScene() (players []*v1.PlayerScene) {
	t.RangePlayer(func(k int32, p *player.Player) bool {
		players = append(players, t.getScene(p))
		return true
	})
	return
}

func (t *Table) getScene(p *player.Player) *v1.PlayerScene {
	if p == nil {
		return nil
	}
	info := &v1.PlayerScene{
		UserID:     p.GetPlayerID(),
		ChairId:    p.GetChairID(),
		Status:     int32(p.GetStatus()),
		Hosting:    p.GetIdleCount() > 0,
		Offline:    p.IsOffline(),
		LastOp:     p.GetLastOp(),
		CurBet:     t.curBet, //
		TotalBet:   p.GetBet(),
		See:        p.IsSee(),
		Cards:      t.getPlayerCards(p),
		IsAutoCall: p.IsAutoCall(),
		IsPaying:   p.IsPaying(),
		CanOp:      t.getPlayerCanOp(p),
	}
	if p.IsSee() {
		info.CurBet = t.curBet * 2
	}
	return info
}

func (t *Table) getPlayerCards(p *player.Player) *v1.CardsInfo {
	c := &v1.CardsInfo{}
	if p.IsSee() {
		c.Hands = p.GetHands()
		c.Type = p.GetCardsType()
	}
	return c
}

func (t *Table) getPlayerCanOp(p *player.Player) (actions []int32) {
	if p == nil {
		return nil
	}

	stage := t.stage.state
	if stage == StWait || stage == StReady || stage == StWaitEnd || stage == StEnd {
		return
	}

	if !p.IsGaming() || len(t.GetCanActionPlayers()) <= 1 {
		return
	}

	// 能否弃牌
	actions = append(actions, AcPack)

	// 能否看牌
	if !t.canSeeCard(p) {
		actions = append(actions, AcSee)
	}

	// 能否主动跟注 call
	canCall, _, canRaise, _ := t.canCallCard(p)
	if canCall {
		actions = append(actions, AcCall)
	}

	// 能否主动加注 Raise
	if canRaise {
		actions = append(actions, AcRaise)
	}

	// 能否主动发起比牌 show
	if canShow, _, _ := t.canShowCard(p); canShow {
		actions = append(actions, AcShow)
	}

	// 能否主动发起提前比牌 side
	if canSideShow, _, _ := t.canSideShowCard(p); canSideShow {
		actions = append(actions, AcSide)
	}

	// 能否 同意/拒绝提前比牌 side_reply
	if t.canSideShowReply(p) {
		actions = append(actions, AcSideReply)
	}
	return actions
}

func (t *Table) canSeeCard(p *player.Player) (canSee bool) {
	if p == nil || p.IsSee() {
		return
	}
	if t.stage.state != StSendCard && t.stage.state != StAction && t.stage.state != StSideShow {
		return
	}
	return true
}

func (t *Table) canPack(p *player.Player) (canPack bool) {
	// 比牌阶段不可丢牌
	if p == nil || !p.IsGaming() || t.stage.state == StSideShow {
		return
	}
	return true
}

// 跟注（Call） 加注（Raise）
func (t *Table) canCallCard(p *player.Player) (canCall bool, callMoney float64, canRaise bool, raiseMoney float64) {
	if p == nil || p.GetChairID() != t.active || t.stage.state != StAction || len(t.GetCanActionPlayers()) <= 2 {
		return
	}
	needMoney := t.calcBetMoney(p)
	if !t.hasEnoughMoney(p, needMoney) {
		return
	}
	raiseMoney = callMoney * 2
	canRaise = t.hasEnoughMoney(p, raiseMoney)
	return true, callMoney, canRaise, raiseMoney
}

// Show
// 当只剩 2 名玩家时，任意一方可请求 Show
// 明牌比较三张牌，胜者赢取全部筹码
func (t *Table) canShowCard(p *player.Player) (canShow bool, showMoney float64, target *player.Player) {
	if p == nil || p.GetChairID() != t.active || t.stage.state != StAction || len(t.GetCanActionPlayers()) != 2 {
		return
	}
	next := t.NextPlayer(p.GetChairID())
	if next == nil {
		return
	}
	needMoney := t.calcBetMoney(p)
	if !t.hasEnoughMoney(p, needMoney) {
		return
	}
	return true, needMoney, next
}

// Side Show
// 剩余玩家数量 > 2
// 仅限明注玩家对上一位明注玩家请求比牌
// 若对方同意，则比大小，小的一方自动弃牌
func (t *Table) canSideShowCard(p *player.Player) (canSideShow bool, sideShowMoney float64, target *player.Player) {
	if p == nil || p.GetChairID() != t.active || t.stage.state != StAction || len(t.GetCanActionPlayers()) <= 2 {
		return
	}
	last := t.LastPlayer(p.GetChairID())
	if last == nil || last == p {
		return
	}
	if !last.IsSee() || !p.IsSee() {
		return
	}
	needMoney := t.calcBetMoney(p)
	if !t.hasEnoughMoney(p, needMoney) {
		return
	}
	return true, needMoney, last
}

// Side Show Reply
// 能否回应提前比牌
func (t *Table) canSideShowReply(p *player.Player) (can bool) {
	if p == nil || p.GetChairID() != t.active || t.stage.state != StSideShow || len(t.GetCanActionPlayers()) <= 2 {
		return
	}
	next := t.NextPlayer(p.GetChairID())
	if next == nil || !next.IsGaming() || !next.IsSee() {
		return
	}
	if !ext.SliceContains(next.GetCompareSeats(), p.GetChairID()) {
		return
	}
	return true
}

// ---------------------------------------------

// BroadcastForwardRsp 消息转发
func (t *Table) BroadcastForwardRsp(ty int32, msg string) {
	t.SendPacketToAll(v1.GameCommand_OnForwardRsp, &v1.ForwardRsp{
		Type: ty,
		Msg:  msg,
	})
}

// 设置庄家推送
func (t *Table) broadcastSetBankerRsp() {
	t.SendPacketToAll(v1.GameCommand_OnSetBankerPush, &v1.SetBankerPush{
		ChairId: t.banker,
	})
}

// 发牌推送
func (t *Table) dispatchCardPush(canGameSeats []*player.Player) {
	t.RangePlayer(func(k int32, p *player.Player) bool {
		t.SendPacketToClient(p, v1.GameCommand_OnSendCardPush, &v1.SendCardPush{
			UserID: p.GetPlayerID(),
			Cards:  t.getPlayerCards(p),
		})
		return true
	})
}

// 广播玩家断线信息
func (t *Table) broadcastUserOffline(p *player.Player) {
	t.SendPacketToAll(v1.GameCommand_OnUserOfflinePush, &v1.UserOfflinePush{
		UserID:    p.GetPlayerID(),
		IsOffline: p.IsOffline(),
	})
}

// 当前活动玩家推送
func (t *Table) broadcastActivePlayerPush() {
	t.RangePlayer(func(k int32, p *player.Player) bool {
		rsp := &v1.ActivePush{
			Stage:    t.stage.state,
			Timeout:  int64(t.calcRemainingTime().Seconds()),
			Active:   t.active,
			CurRound: t.curRound,
			CurBet:   t.curBet,
			TotalBet: t.totalBet,
			RaiseBet: t.curBet * 2,
		}
		if p.GetChairID() == t.active {
			rsp.CanOp = t.getPlayerCanOp(t.GetActivePlayer())
		}
		t.SendPacketToClient(p, v1.GameCommand_OnActivePush, rsp)
		return true
	})
}

// 玩家离桌推送
func (t *Table) broadcastUserQuitPush(p *player.Player, isSwitchTable bool) {
	t.SendPacketToAllExcept(v1.GameCommand_OnPlayerQuitPush, &v1.PlayerQuitPush{
		UserID:  p.GetPlayerID(),
		ChairID: p.GetChairID(),
	}, p.GetPlayerID())
}

func (t *Table) sendActionRsp(p *player.Player, rsp *v1.ActionRsp) {
	t.SendPacketToClient(p, v1.GameCommand_OnActionRsp, rsp)
}

func (t *Table) sendActiveButtonInfoNtf() {
	active := t.GetActivePlayer()
	if active == nil {
		return
	}
	canShow, _, _ := t.canShowCard(active)
	canSideShow, _, _ := t.canSideShowCard(active)
	if canShow || canSideShow {
		t.SendPacketToClient(active, v1.GameCommand_OnAfterSeeButtonPush, &v1.AfterSeeButtonPush{
			PlayerID:    active.GetPlayerID(),
			CanShow:     canShow,
			CanSideShow: canSideShow,
		})
	}
}

func (t *Table) broadcastActionRsp(p *player.Player, action int32, playerBet float64, target *player.Player, allow bool) {
	rsp := &v1.ActionRsp{
		Code:        0,
		Msg:         "",
		UserID:      p.GetPlayerID(),
		ChairID:     p.GetChairID(),
		Action:      action,
		SeeCards:    nil,
		BetInfo:     nil,
		CompareInfo: nil,
	}
	// 看牌
	if action == AcSee {
		t.SendPacketToAllExcept(v1.GameCommand_OnActionRsp, rsp, p.GetPlayerID())
		rsp.SeeCards = &v1.SeeCards{
			Cards:     p.GetHands(),
			CardsType: p.GetCardsType(),
		}
		t.SendPacketToClient(p, v1.GameCommand_OnActionRsp, rsp)
		return
	}
	// 下注
	rsp.BetInfo = &v1.BetInfo{
		CurBet:    t.curBet,
		TotalBet:  t.totalBet,
		PlayerBet: playerBet,
	}
	// 比牌
	if target != nil && (action == AcShow || action == AcSide || action == AcSideReply) {
		rsp.CompareInfo = &v1.CompareInfo{
			TargetUid:      target.GetPlayerID(),
			TargetChairID:  target.GetChairID(),
			TargetStatus:   int32(target.GetStatus()),
			SideReplyAllow: allow,
		}
	}
	t.SendPacketToAll(v1.GameCommand_OnActionRsp, rsp)

	// see 4011
	// 		see cards
	// 		canShow bool

	// call/Raise 4011
	// 		"uid":       seat.UID,
	//		"seatid":    seat.SID,
	//		"money":     seat.GetSelfMoney(),
	//		"action":    PLAYER_CALL,
	//		"bet":       seat.Bet,
	//		"cur_bet":   bet,
	//		"bet_ratio": 1, // 2
	//		"total_bet": t.TotalBet,
	//

	// show 4011
	//		"uid":           curr.UID,
	//		"seatid":        curr.SID,
	//		"status":        curr.Status,
	//		"money":         curr.GetSelfMoney(),
	//		"action":        PLAYER_SHOW,
	//		"bet":           curr.Bet,
	//		"target_uid":    last.UID,
	//		"target_seatid": last.SID,
	//		"target_status": last.Status,
	//		"type":          0,
	//		"cur_bet":       bet,
	//		"total_bet":     t.TotalBet,

	// side show 4011
	//		"uid":           curr.UID,
	//		"seatid":        curr.SID,
	//		"status":        curr.Status,
	//		"money":         curr.GetSelfMoney(),
	//		"action":        PLAYER_SHOW,
	//		"bet":           curr.Bet,
	//		"target_uid":    last.UID,
	//		"target_seatid": last.SID,
	//		"target_status": last.Status,
	//		"type":          0,
	//		"cur_bet":       bet,
	//		"total_bet":     t.TotalBet,

	// side show reply
	//			"uid":           last.UID,
	//			"seatid":        last.SID,
	//			"status":        last.Status,
	//			"money":         last.GetSelfMoney(),
	//			"action":        PLAYER_SHOW,
	//			"bet":           last.Bet,
	//			"target_uid":    curr.UID,
	//			"target_seatid": curr.SID,
	//			"target_status": curr.Status,
	//			"type":          1,
	//			"cur_bet":       t.CurBet * 2,
	//			"total_bet":     t.TotalBet,

	// t.SendPacketToAll(v1.GameCommand_OnActionRsp, rsp)
}
