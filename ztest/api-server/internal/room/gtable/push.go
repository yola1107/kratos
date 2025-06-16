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

func (t *Table) BroadcastUserInfo(p *gplayer.Player) {
	t.SendUserInfoToAnother(p, p)
	for k, v := range t.seats {
		if v != nil && k != int(p.GetChairID()) {
			t.SendUserInfoToAnother(p, v)
			t.SendUserInfoToAnother(v, p)
		}
	}
}

func (t *Table) SendUserInfoToAnother(src *gplayer.Player, dst *gplayer.Player) {
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
		UserID:   p.GetPlayerID(),
		ChairId:  p.GetChairID(),
		Status:   int32(p.GetStatus()),
		Hosting:  p.IsHosting(),
		Offline:  p.IsOffline(),
		LastOp:   p.GetLastOp(),
		CurBet:   t.curBet, //
		TotalBet: p.GetBet(),
		See:      p.GetSee(),
		Cards:    t.getPlayerCards(p),
		AutoCall: p.GetAutoCall(),
		Paying:   p.IsPaying(),
		CanOp:    t.getPlayerCanOp(p),
	}
	if p.GetSee() > 0 {
		info.CurBet = t.curBet * 2
	}
	return info
}

func (t *Table) getPlayerCards(p *gplayer.Player) *v1.CardsInfo {
	c := &v1.CardsInfo{}
	if p.GetSee() > 0 {
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

func (t *Table) BroadcastUserExit(p *gplayer.Player) {

}

func (t *Table) broadcastForwardRsp(ty int32, msg string) {
	t.SendPacketToAll(v1.GameCommand_OnForwardRsp, &v1.ForwardRsp{
		Type: ty,
		Msg:  msg,
	})
}

func (t *Table) broadcastSetBankerRsp() {
	t.SendPacketToAll(v1.GameCommand_OnSetBankerPush, &v1.SetBankerPush{
		ChairId: t.banker,
	})
}

func (t *Table) dispatchCardPush(canGameSeats []*gplayer.Player) {
	t.RangePlayer(func(k int32, p *gplayer.Player) bool {
		t.SendPacketToClient(p, v1.GameCommand_OnSendCardPush, &v1.SendCardPush{
			UserID: p.GetPlayerID(),
			Cards:  t.getPlayerCards(p),
		})
		return true
	})
}

func (t *Table) broadcastActivePlayerPush() {
	t.SendPacketToAll(v1.GameCommand_OnActivePush, &v1.ActivePush{
		Stage:    t.stage.state,
		Timeout:  int64(t.calcRemainingTime().Seconds()),
		Active:   t.active,
		CurRound: t.curRound,
		Player:   t.getScene(t.GetActivePlayer()),
	})
}
