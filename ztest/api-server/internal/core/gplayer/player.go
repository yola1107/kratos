package gplayer

type Player struct {
	session  Session
	gameData gameData
	baseData baseData // 私有，不暴露
	chipData chipData // 私有，不暴露
}
type chipData struct{}

func (p *Player) Reset() {
}

func (p *Player) SetUID(uid int64) {}

func (p *Player) GetUID() int64 {
	return p.baseData.UID
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
