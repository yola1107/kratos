package gplayer

import (
	"github.com/yola1107/kratos/v2/transport/websocket"
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

func (p *Player) SetHosting(hosting bool) {
	return
}

func (p *Player) IsHosting() bool {
	return false
}

func (p *Player) SetOffline() {
}

func (p *Player) IsOffline() bool {
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
