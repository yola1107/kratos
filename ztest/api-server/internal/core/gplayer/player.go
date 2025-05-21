package gplayer

type Player struct {
	session  Session
	gameData PlayerGameData
	baseData PlayerBaseData // 私有，不暴露
}

func (p *Player) GetSession() Session {
	return p.session
}
func (p *Player) GetGameData() PlayerGameData {
	return p.gameData
}
func (p *Player) GetBaseData() PlayerBaseData {
	return p.baseData
}

func (p *Player) Reset() {
}

func (p *Player) SetPlayerID(uid int64) {}

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

func (p *Player) SaveBaseDataToDB() {
}

func (p *Player) LoadBaseDataFromDB() {
}
