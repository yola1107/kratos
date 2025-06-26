package table

import (
	"github.com/golang/protobuf/proto"
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
		t.aiLogic.OnMessage(p, cmd, msg)
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

	// 记录进桌时间
	t.aiLogic.markEnterTime()
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
		Money:     src.GetAllMoney(),
		Avatar:    src.GetAvatar(),
		AvatarUrl: src.GetAvatarUrl(),
		Vip:       src.GetVipGrade(),
		Status:    int32(src.GetStatus()),
		Ip:        src.GetIP(),
	})
}

// BroadcastForwardRsp 消息转发
func (t *Table) BroadcastForwardRsp(ty int32, msg string) {
	t.SendPacketToAll(v1.GameCommand_OnForwardRsp, &v1.ForwardRsp{
		Type: ty,
		Msg:  msg,
	})
}

// 广播玩家断线信息
func (t *Table) broadcastUserOffline(p *player.Player) {
	t.SendPacketToAll(v1.GameCommand_OnUserOfflinePush, &v1.UserOfflinePush{
		UserID:    p.GetPlayerID(),
		IsOffline: p.IsOffline(),
	})
}

// 玩家离桌推送
func (t *Table) broadcastUserQuitPush(p *player.Player, isSwitchTable bool) {
	t.SendPacketToAllExcept(v1.GameCommand_OnPlayerQuitPush, &v1.PlayerQuitPush{
		UserID:  p.GetPlayerID(),
		ChairID: p.GetChairID(),
	}, p.GetPlayerID())

	// 记录时间
	t.aiLogic.markExitTime()
}

// ---------------------------------------------
/*
	游戏协议
*/

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
			UserID:    p.GetPlayerID(),
			Cards:     p.GetCards(),
			CardsType: p.GetCardsType(),
		})
		return true
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
		Stage:        int32(t.stage.State),
		Timeout:      int64(t.stage.Remaining().Seconds()),
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
		Hosting:    p.GetTimeoutCnt() > 0,
		Offline:    p.IsOffline(),
		LastOp:     p.GetLastOp(),
		CurBet:     t.curBet, //
		TotalBet:   p.GetBet(),
		Seen:       p.Seen(),
		Cards:      p.GetCards(),
		CardsType:  p.GetCardsType(),
		IsAutoCall: p.IsAutoCall(),
		IsPaying:   p.IsPaying(),
		CanOp:      t.getCanOp(p),
	}
	if p.Seen() {
		info.CurBet = t.curBet * 2
	}
	return info
}

// 当前活动玩家推送
func (t *Table) broadcastActivePlayerPush() {

	playerCnt := len(t.GetGamers())
	if playerCnt <= 1 {
		return
	}

	t.RangePlayer(func(k int32, p *player.Player) bool {
		rsp := &v1.ActivePush{
			Stage:    int32(t.stage.State),
			Timeout:  int64(t.stage.Remaining().Seconds()),
			Active:   t.active,
			CurRound: t.curRound,
			CurBet:   t.curBet,
			TotalBet: t.totalBet,
			RaiseBet: t.curBet * 2,
		}
		if p.GetChairID() == t.active && p.IsGaming() {
			rsp.CanOp = t.getCanOp(p)
			t.mLog.activePush(p, t.first, t.curRound, rsp.CanOp, playerCnt)
			if len(rsp.CanOp) == 0 {
				log.Errorf("ActivePush empty. tb:%v, p:%v", t.Desc(), p.Desc())
			}
			// log.Debugf("ActivePush. p:%v CanOp=%v, tb:%v ", p.Desc(), rsp.CanOp, t.Desc())
		}
		t.SendPacketToClient(p, v1.GameCommand_OnActivePush, rsp)
		return true
	})
}

func (t *Table) sendActiveButtonInfoNtf() {
	active := t.GetActivePlayer()
	if active == nil {
		return
	}
	canShow := t.canShowCard(active).Code == ErrOK
	canSideShow := t.canSideShowCard(active).Code == ErrOK
	if canShow || canSideShow {
		t.SendPacketToClient(active, v1.GameCommand_OnAfterSeeButtonPush, &v1.AfterSeeButtonPush{
			UserID:      active.GetPlayerID(),
			CanShow:     canShow,
			CanSideShow: canSideShow,
		})
	}
}

func (t *Table) sendActionRsp(p *player.Player, rsp *v1.ActionRsp) {
	t.SendPacketToClient(p, v1.GameCommand_OnActionRsp, rsp)
}

func (t *Table) broadcastActionRsp(p *player.Player, action v1.ACTION, playerBet float64, target *player.Player, allow bool) {
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
	if action == v1.ACTION_SEE {
		t.SendPacketToAllExcept(v1.GameCommand_OnActionRsp, rsp, p.GetPlayerID())
		rsp.SeeCards = &v1.SeeCards{
			Cards:     p.GetCards(),
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
	if target != nil && (action == v1.ACTION_SHOW || action == v1.ACTION_SIDE || action == v1.ACTION_SIDE_REPLY) {
		rsp.CompareInfo = &v1.CompareInfo{
			TargetUid:      target.GetPlayerID(),
			TargetChairID:  target.GetChairID(),
			TargetStatus:   int32(target.GetStatus()),
			SideReplyAllow: allow,
		}
	}
	t.SendPacketToAll(v1.GameCommand_OnActionRsp, rsp)
}

func (t *Table) getCanOp(p *player.Player) (actions []v1.ACTION) {
	if p == nil {
		return nil
	}

	if !p.IsGaming() || len(t.GetGamers()) <= 1 {
		return
	}

	stage := t.stage.State
	if stage == StWait || stage == StReady || stage == StWaitEnd || stage == StEnd {
		return
	}

	// 能否弃牌
	if t.canPack(p).Code == ErrOK {
		actions = append(actions, v1.ACTION_PACK)
	}

	// 能否看牌
	if t.canSeeCard(p).Code == ErrOK {
		actions = append(actions, v1.ACTION_SEE)
	}

	// 能否主动跟注 call
	if t.canCallCard(p, false).Code == ErrOK {
		actions = append(actions, v1.ACTION_CALL)
	}

	// 能否主动加注 Raise
	if t.canCallCard(p, true).Code == ErrOK {
		actions = append(actions, v1.ACTION_RAISE)
	}

	// 能否主动发起比牌 show
	if t.canShowCard(p).Code == ErrOK {
		actions = append(actions, v1.ACTION_SHOW)
	}

	// 能否主动发起提前比牌 side
	if t.canSideShowCard(p).Code == ErrOK {
		actions = append(actions, v1.ACTION_SIDE)
	}

	// 能否 同意/拒绝提前比牌 side_reply
	if t.canSideShowReply(p).Code == ErrOK {
		actions = append(actions, v1.ACTION_SIDE_REPLY)
	}

	return actions
}

func (t *Table) broadcastResult() {
	t.SendPacketToAll(v1.GameCommand_OnResultPush, &v1.ResultPush{
		UserID:  0,
		ChairID: 0,
	})
}
