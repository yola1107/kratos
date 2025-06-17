package gplayer

import (
	"github.com/golang/protobuf/proto"
	"github.com/yola1107/kratos/v2/errors"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
)

type Player struct {
	session  *websocket.Session
	gameData PlayerGameData
	baseData PlayerBaseData // 私有，不暴露
}

func (p *Player) IsRobot() bool {
	// return p.PlayerBase.IsRobot
	return false
}

func (p *Player) GetSessionID() string {
	if p.session == nil {
		return ""
	}
	return p.session.ID()
}

func (p *Player) GetSession() *websocket.Session {
	return p.session
}

func (p *Player) UpdateSession(session *websocket.Session) {
	p.session = session
}

func (p *Player) push(cmd v1.GameCommand, msg proto.Message) {
	if p == nil {
		return
	}
	if p.IsRobot() {
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

func (p *Player) SendSwitchTableRsp(e *errors.Error) {
	if p == nil {
		return
	}
	code, msg := int32(0), ""
	if e != nil {
		code, msg = e.Code, e.Message
	}
	p.push(v1.GameCommand_OnSwitchTableRsp, &v1.SwitchTableRsp{
		Code:   code,
		Msg:    msg,
		UserID: p.GetPlayerID(),
	})
}
