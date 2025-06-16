package gplayer

import (
	"fmt"
)

var (
	StFree     = Status(0)
	StSit      = Status(1)
	StReady    = Status(2)
	StGaming   = Status(3)
	StGameFold = Status(4)
	StGameLost = Status(5)
)

type Status int32

type PlayerGameData struct {
	IsReady    bool
	Status     Status     // 0 StFree 1 StSit 2 StReady 3 StGaming
	isOffline  bool       // 是否离线
	isHosting  bool       // 是否为托管状态
	Bet        float64    // 投注
	LastOp     int32      // 上一次操作
	See        int32      // 是否看牌
	cards      *cardsInfo //
	PlayCount  int32      // 玩的回合数
	SeeRound   int32      // 看牌回合数
	StartMoney float64    // 局数开始时的金币
	IdleCount  int32      // 超时次数

	CompareSeats []int // 比牌椅子号
	IsAllCompare bool  // 是否参与所有比牌

	AutoCall int32 // 是否自动跟注 0：未开启自动跟注 1：开启了自动跟注
	Paying   int32 // 支付中

}

type cardsInfo struct {
	hand []int32 // 当前的手牌
	outs []int32 // 出牌列表
	ty   int32   // 牌型
}

func (c *cardsInfo) AddCards(cards []int32) {
	c.hand = append(c.hand, cards...)
}

func (c *cardsInfo) OutCards(cards ...int32) {
	// ...
}

func (p *Player) Reset() {
	p.gameData.Bet = 0
	p.gameData.Status = 0
	p.gameData.LastOp = 0
	p.gameData.See = 0
	p.gameData.cards = &cardsInfo{}
	p.gameData.PlayCount = 0
	p.gameData.SeeRound = 0
	p.gameData.StartMoney = 0
	p.gameData.IsAllCompare = false
	p.gameData.AutoCall = 0
	p.gameData.Paying = 0
}

func (p *Player) Desc() string {
	return fmt.Sprintf("(%d %d T:%d M:%.1f B:%.1f S:%d)",
		p.GetPlayerID(), p.GetChairID(), p.GetTableID(), p.GetMoney(), p.GetBet(), p.GetSee())
}

func (p *Player) SetStatus(status Status) {
	p.gameData.Status = status
}

func (p *Player) GetStatus() Status {
	return p.gameData.Status
}

func (p *Player) SetHosting(hosting bool) {
	return
}

func (p *Player) IsHosting() bool {
	return false
}

func (p *Player) SetOffline(offline bool) {
	p.gameData.isOffline = offline
}

func (p *Player) IsOffline() bool {
	return p.gameData.isOffline
}

func (p *Player) IsSited() bool {
	return p.gameData.Status == StSit
}

func (p *Player) IsReady() bool {
	return p.gameData.Status == StReady
}

func (p *Player) IsGaming() bool {
	return p.gameData.Status == StGaming
}

func (p *Player) GetLastOp() int32 {
	return p.gameData.LastOp
}

func (p *Player) GetBet() float64 {
	return p.gameData.Bet
}

func (p *Player) GetSee() int32 {
	return p.gameData.See
}

func (p *Player) GetAutoCall() int32 {
	return p.gameData.AutoCall
}

func (p *Player) IsPaying() int32 {
	return p.gameData.Paying
}

func (p *Player) GetHands() []int32 {
	return p.gameData.cards.hand
}

func (p *Player) GetOuts() []int32 {
	return p.gameData.cards.outs
}

func (p *Player) AddCards(cs []int32) {
	p.gameData.cards.AddCards(cs)
}

func (p *Player) OutCards(cs []int32) {
	p.gameData.cards.OutCards(cs...)
}

func (p *Player) GetCardsType() int32 {
	return p.gameData.cards.ty
}

func (p *Player) IntoGaming(bet float64) bool {
	if !p.UseMoney(bet) {
		return false
	}
	p.gameData.StartMoney = p.GetMoney()
	p.gameData.Bet += bet
	p.SetStatus(StGaming)
	return true
}
