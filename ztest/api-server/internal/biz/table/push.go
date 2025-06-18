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

func (t *Table) SendPacketToAll(cmd v1.GameCommand, msg proto.Message, uids ...int64) {
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

func (t *Table) getPlayerCanOp(p *player.Player) (actions []v1.ACTION) {
	if p == nil {
		return nil
	}

	stage := t.stage.state
	if stage == conf.StWait || stage == conf.StReady || stage == conf.StWaitEnd || stage == conf.StEnd {
		return
	}

	if !p.IsGaming() || len(t.GetCanActionPlayers()) <= 1 {
		return
	}

	// 能否弃牌
	actions = append(actions, v1.ACTION_PACK)

	// 能否看牌
	if !t.canSeeCard(p) {
		actions = append(actions, v1.ACTION_SEE)
	}

	// 能否主动跟注 call
	canCall, _, canRaise, _ := t.canCallCard(p)
	if canCall {
		actions = append(actions, v1.ACTION_CALL)
	}

	// 能否主动加注 Raise
	if canRaise {
		actions = append(actions, v1.ACTION_RAISE)
	}

	// 能否主动发起比牌 show
	if canShow, _ := t.canShowCard(p); canShow {
		actions = append(actions, v1.ACTION_SHOW)
	}

	// 能否主动发起提前比牌 side_show
	if canSideShow, _, _ := t.canSideShowCard(p); canSideShow {
		actions = append(actions, v1.ACTION_SIDE)
	}

	// 能否 同意/拒绝提前比牌 side_show_reply
	if t.canSideShowReply(p) {
		actions = append(actions, v1.ACTION_SIDE_REPLY)
	}
	return actions
}

func (t *Table) canSeeCard(p *player.Player) (canSee bool) {
	if p.IsSee() || t.stage.state != conf.StSendCard && t.stage.state != conf.StAction && t.stage.state != conf.StSideShow {
		return
	}
	return true
}

// 跟注（Call） 加注（Raise）
func (t *Table) canCallCard(p *player.Player) (canCall bool, callMoney float64, canRaise bool, raiseMoney float64) {
	if p.GetChairID() != t.active || t.stage.state != conf.StAction || len(t.GetCanActionPlayers()) <= 2 {
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
func (t *Table) canShowCard(p *player.Player) (canShow bool, showMoney float64) {
	if p.GetChairID() != t.active || t.stage.state != conf.StAction || len(t.GetCanActionPlayers()) != 2 {
		return
	}
	needMoney := t.calcBetMoney(p)
	if !t.hasEnoughMoney(p, needMoney) {
		return
	}
	return true, needMoney
}

// Side Show
// 剩余玩家数量 > 2
// 仅限明注玩家对上一位明注玩家请求比牌
// 若对方同意，则比大小，小的一方自动弃牌
func (t *Table) canSideShowCard(p *player.Player) (canSideShow bool, sideShowMoney float64, target *player.Player) {
	if p.GetChairID() != t.active || t.stage.state != conf.StAction || len(t.GetCanActionPlayers()) <= 2 {
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
	if p.GetChairID() != t.active || t.stage.state != conf.StSideShow || len(t.GetCanActionPlayers()) <= 2 {
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
	// t.SendPacketToAll(v1.GameCommand_OnActivePush, &v1.ActivePush{
	// 	Stage:    t.stage.state,
	// 	Timeout:  int64(t.calcRemainingTime().Seconds()),
	// 	Active:   t.active,
	// 	CurRound: t.curRound,
	// 	Player:   t.getScene(t.GetActivePlayer()),
	// })

	rsp := &v1.ActivePush{
		Stage:    t.stage.state,
		Timeout:  int64(t.calcRemainingTime().Seconds()),
		Active:   t.active,
		CurRound: t.curRound,
		CurBet:   t.curBet,
		TotalBet: t.totalBet,
		CanOp:    t.getPlayerCanOp(t.GetActivePlayer()),
		// Player:   t.getScene(t.GetActivePlayer()),
	}
	t.RangePlayer(func(k int32, p *player.Player) bool {
		if p.GetChairID() != t.active {
			t.SendPacketToClient(p, v1.GameCommand_OnActivePush, rsp)
		} else {

		}
		return true
	})
}

// 玩家离桌推送
func (t *Table) broadcastUserQuitPush(p *player.Player, isSwitchTable bool) {
	t.SendPacketToAll(v1.GameCommand_OnPlayerQuitPush, &v1.PlayerQuitPush{
		UserID:  p.GetPlayerID(),
		ChairID: p.GetChairID(),
	}, p.GetPlayerID())
}

func (t *Table) sendActionRsp(p *player.Player, rsp *v1.ActionRsp) {
	t.SendPacketToClient(p, v1.GameCommand_OnActionRsp, rsp)
}

func (t *Table) broadcastActionRsp(p *player.Player, action v1.ACTION) {
	rsp := &v1.ActionRsp{
		Code:   0,
		Msg:    "",
		UserID: p.GetPlayerID(),
		Action: action,
		Cards:  nil,
	}

	t.SendPacketToAll(v1.GameCommand_OnActionRsp, rsp)
}
