package gtable

import (
	"github.com/golang/protobuf/proto"
	"github.com/yola1107/kratos/v2/log"

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

func (t *Table) BroadcastUserEnter(p *gplayer.Player) {}

func (t *Table) SendTableInfo(p *gplayer.Player) {}

func (t *Table) BroadcastUserExit(p *gplayer.Player) {}
