package gplayer

type Player struct {
	session  Session
	gameData gameData
	baseData baseData // 私有，不暴露
	chipData chipData // 私有，不暴露
}

type Session struct{}

type gameData struct {
	// 游戏过程数据
}

type baseData struct {
	id    int64
	level int32
}

type chipData struct{}

func (p *Player) Reset() {

}

func (p *Player) GetID() int64 {
	return p.baseData.id
}

func (p *Player) SetLevel(level int32) {
	p.baseData.level = level
}
