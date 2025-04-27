package gplayer

import (
	"github.com/yola1107/kratos/v2/transport/websocket"
)

type Player struct {
	session  *websocket.Session
	gameData PlayerGameData
	baseData PlayerBaseData // 私有，不暴露
}

func (p *Player) Reset() {
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

func (p *Player) GetMoney() float64 {
	return 0
}

func (p *Player) GetVipGrade() int32 {
	return p.baseData.VIP
}

func (p *Player) GetPlayerID() int64 {
	return p.baseData.UID
}

func (p *Player) GetTableID() (TableID int32) {
	return p.baseData.TID
}

func (p *Player) SetTableID(tableID int32) {
	p.baseData.TID = tableID
}

func (p *Player) GetChairID() (ChairID int32) {
	return p.gameData.SID
}

func (p *Player) SetChairID(ChairID int32) {
	p.gameData.SID = ChairID
	return
}

func (p *Player) SetOffline() {
}

func (p *Player) IsOffline() bool {
	return false
}

func (p *Player) IsRobot() bool {
	// return p.PlayerBase.IsRobot
	return false
}

func (p *Player) SaveBaseDataToDB() {
}

func (p *Player) LoadBaseDataFromDB() {
}
