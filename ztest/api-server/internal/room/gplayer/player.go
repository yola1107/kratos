package gplayer

type Player struct {
	session  Session
	gameData gameData
	baseData baseData // 私有，不暴露
	chipData chipData // 私有，不暴露
}

func (p *Player) Reset() {
}

func (p *Player) GetID() int64 {
	return 0
}

func (p *Player) GetTableID() (TableID int32) {
	return p.baseData.TableID
}
