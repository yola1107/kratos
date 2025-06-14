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

}

func (t *Table) SendSceneInfo(p *gplayer.Player) {
	rsp := &v1.SceneRsp{
		Stage:         t.stage.state,
		ActiveChairId: t.active,
		RemainingTime: t.calcRemainingTime().Milliseconds(),
		BankerId:      0,
		Players:       nil,
		ArenaID:       int32(conf.ArenaID),
		SN:            0,
	}

	t.SendPacketToClient(p, v1.GameCommand_OnSceneRsp, rsp)
}

func (t *Table) BroadcastUserExit(p *gplayer.Player) {

}
