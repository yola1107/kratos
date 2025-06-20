package player

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

type GameData struct {
	TableID      int32      // 桌子ID
	ChairID      int32      // 椅子ID
	Status       Status     // 0 StFree 1 StSit 2 StReady 3 StGaming
	isOffline    bool       // 是否离线
	Bet          float64    // 投注
	LastOp       int32      // 上一次操作
	isSee        bool       // 是否看牌
	cards        *cardsInfo // 手牌
	playCount    int32      // 玩的回合数
	seeRound     int32      // 看牌回合数
	startMoney   float64    // 局数开始时的金币
	idleCount    int32      // 超时/托管次数
	compareSeats []int32    // 比牌椅子号
	isAllCompare bool       // 是否参与所有比牌
	isAutoCall   bool       // 是否自动跟注 0：未开启自动跟注 1：开启了自动跟注
	isPaying     bool       // 支付中
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
	p.gameData.Status = 0
	p.gameData.isOffline = false
	p.gameData.Bet = 0
	p.gameData.LastOp = 0
	p.gameData.isSee = false
	p.gameData.cards = &cardsInfo{}
	p.gameData.playCount = 0
	p.gameData.seeRound = 0
	p.gameData.startMoney = 0
	p.gameData.idleCount = 0
	p.gameData.compareSeats = nil
	p.gameData.isAllCompare = false
	p.gameData.isAutoCall = false
	p.gameData.isPaying = false
}

func (p *Player) ExitReset() {
	p.Reset()
	p.gameData.ChairID = -1
	p.gameData.TableID = -1
	p.gameData.Status = -1

	// 计算金币
	// p.PlayerBase.GameHallData.SaveMoney(int64(p.chouMa.GetDeltaMoney()))
	// p.chouMa.ResetDelta()
}

func (p *Player) Desc() string {
	see := 0
	if p.gameData.isSee {
		see = 1
	}
	return fmt.Sprintf("(%d %d T:%d M:%.1f B:%.1f S:%d)",
		p.GetPlayerID(), p.GetChairID(), p.GetTableID(), p.GetMoney(), p.GetBet(), see)
}

func (p *Player) SetTableID(tableID int32) {
	p.gameData.TableID = tableID
}

func (p *Player) GetTableID() (TableID int32) {
	return p.gameData.TableID
}

func (p *Player) SetChairID(ChairID int32) {
	p.gameData.ChairID = ChairID
	return
}

func (p *Player) GetChairID() (ChairID int32) {
	return p.gameData.ChairID
}

func (p *Player) SetStatus(status Status) {
	p.gameData.Status = status
}

func (p *Player) GetStatus() Status {
	return p.gameData.Status
}

func (p *Player) IncrIdleCount(isTimeout bool) {
	if !isTimeout {
		return
	}
	p.gameData.idleCount++
}

func (p *Player) ClearIdleCount() {
	p.gameData.idleCount = 0
}

func (p *Player) GetIdleCount() int32 {
	return p.gameData.idleCount
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

func (p *Player) SetLastOp(op int32) {
	p.gameData.LastOp = op
}
func (p *Player) GetLastOp() int32 {
	return p.gameData.LastOp
}

// ---------------------------------

func (p *Player) AddBet(bet float64) {
	p.gameData.Bet += bet
}

func (p *Player) GetBet() float64 {
	return p.gameData.Bet
}

// ---------------------------------

func (p *Player) SetSee() {
	p.gameData.isSee = true
}

func (p *Player) IsSee() bool {
	return p.gameData.isSee
}

func (p *Player) IsAutoCall() bool {
	return p.gameData.isAutoCall
}

func (p *Player) IsPaying() bool {
	return p.gameData.isPaying
}

func (p *Player) GetCards() []int32 {
	return p.gameData.cards.hand
}

func (p *Player) GetCardsType() int32 {
	return p.gameData.cards.ty
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

func (p *Player) IntoGaming(bet float64) bool {
	if !p.UseMoney(bet) {
		return false
	}
	p.gameData.startMoney = p.GetMoney()
	p.gameData.Bet += bet
	p.SetStatus(StGaming)
	return true
}

func (p *Player) IncrPlayCount() {
	p.gameData.playCount++
}

func (p *Player) SetCompareSeats(chairs []int32) {
	p.gameData.compareSeats = chairs
}

func (p *Player) GetCompareSeats() []int32 {
	return p.gameData.compareSeats
}
