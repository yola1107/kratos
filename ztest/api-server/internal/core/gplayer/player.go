package gplayer

type Player struct {
	session  Session
	gameData gameData
	baseData baseData // 私有，不暴露
	chipData chipData // 私有，不暴露
}

type Session struct {
	ID string
}

type chipData struct {
}

func (p *Player) Reset() {
}

func (p *Player) SetPlayerID(uid int64) {}

func (p *Player) GetPlayerID() int64 {
	return p.baseData.ID
}

func (p *Player) GetTableID() (TableID int32) {
	return p.baseData.TableID
}

func (p *Player) SetTableID(tableID int32) {
	p.baseData.TableID = tableID
}

func (p *Player) GetChairID() (ChairID int32) {
	return p.gameData.ChairID
}

func (p *Player) SetChairID(ChairID int32) {
	p.gameData.ChairID = ChairID
	return
}

func (p *Player) SaveBaseDataToDB() {
}

func (p *Player) LoadBaseDataFromDB() {
}
