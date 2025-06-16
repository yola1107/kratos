package gtable

import (
	"github.com/golang/protobuf/proto"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"

	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gplayer"
)

func (t *Table) SendPacketToClient(p *gplayer.Player, cmd v1.GameCommand, msg proto.Message) {
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
func (t *Table) SendLoginRsp(p *gplayer.Player, code int32, msg string) {
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
func (t *Table) broadcastUserInfo(p *gplayer.Player) {
	t.sendUserInfoToAnother(p, p)
	for k, v := range t.seats {
		if v != nil && k != int(p.GetChairID()) {
			t.sendUserInfoToAnother(p, v)
			t.sendUserInfoToAnother(v, p)
		}
	}
}

func (t *Table) sendUserInfoToAnother(src *gplayer.Player, dst *gplayer.Player) {
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
func (t *Table) SendSceneInfo(p *gplayer.Player) {
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
	t.RangePlayer(func(k int32, p *gplayer.Player) bool {
		players = append(players, t.getScene(p))
		return true
	})
	return
}

func (t *Table) getScene(p *gplayer.Player) *v1.PlayerScene {
	if p == nil {
		return nil
	}
	info := &v1.PlayerScene{
		UserID:     p.GetPlayerID(),
		ChairId:    p.GetChairID(),
		Status:     int32(p.GetStatus()),
		Hosting:    p.IsHosting(),
		Offline:    p.IsOffline(),
		LastOp:     p.GetLastOp(),
		CurBet:     t.curBet, //
		TotalBet:   p.GetBet(),
		See:        p.IstSee(),
		Cards:      t.getPlayerCards(p),
		IsAutoCall: p.IsAutoCall(),
		IsPaying:   p.IsPaying(),
		CanOp:      t.getPlayerCanOp(p),
	}
	if p.IstSee() {
		info.CurBet = t.curBet * 2
	}
	return info
}

func (t *Table) getPlayerCards(p *gplayer.Player) *v1.CardsInfo {
	c := &v1.CardsInfo{}
	if p.IstSee() {
		c.Hands = p.GetHands()
		c.Type = p.GetCardsType()
	}
	return c
}

func (t *Table) getPlayerCanOp(p *gplayer.Player) []v1.Action {
	if p == nil {
		return nil
	}
	return nil
}

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
func (t *Table) dispatchCardPush(canGameSeats []*gplayer.Player) {
	t.RangePlayer(func(k int32, p *gplayer.Player) bool {
		t.SendPacketToClient(p, v1.GameCommand_OnSendCardPush, &v1.SendCardPush{
			UserID: p.GetPlayerID(),
			Cards:  t.getPlayerCards(p),
		})
		return true
	})
}

// 广播玩家断线信息
func (t *Table) broadcastUserOffline(p *gplayer.Player) {
	t.SendPacketToAll(v1.GameCommand_OnUserOfflinePush, &v1.UserOfflinePush{
		UserID:    p.GetPlayerID(),
		IsOffline: p.IsOffline(),
	})
}

// 当前活动玩家推送
func (t *Table) broadcastActivePlayerPush() {
	t.SendPacketToAll(v1.GameCommand_OnActivePush, &v1.ActivePush{
		Stage:    t.stage.state,
		Timeout:  int64(t.calcRemainingTime().Seconds()),
		Active:   t.active,
		CurRound: t.curRound,
		Player:   t.getScene(t.GetActivePlayer()),
	})
}

// 玩家离桌推送
func (t *Table) broadcastUserQuitPush(p *gplayer.Player, isSwitchTable bool) {
	t.SendPacketToAll(v1.GameCommand_OnPlayerQuitPush, &v1.PlayerQuitPush{
		UserID:  p.GetPlayerID(),
		ChairID: p.GetChairID(),
	}, p.GetPlayerID())
}
