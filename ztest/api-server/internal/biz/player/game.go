package player

import (
	"fmt"

	"github.com/yola1107/kratos/v2/log"
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
	status       Status     // 0 StFree 1 StSit 2 StReady 3 StGaming
	isOffline    bool       // 是否离线
	bet          float64    // 投注
	lastOp       int32      // 上一次操作
	seen         bool       // 是否看牌
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
	p.gameData.status = 0
	p.gameData.isOffline = false
	p.gameData.bet = 0
	p.gameData.lastOp = 0
	p.gameData.seen = false
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
	p.gameData.status = -1

	// 计算金币
	// p.PlayerBase.GameHallData.SaveMoney(int64(p.chouMa.GetDeltaMoney()))
	// p.chouMa.ResetDelta()
}

func (p *Player) Desc() string {
	see := 0
	if p.gameData.seen {
		see = 1
	}
	ai := 0
	if p.isRobot {
		ai = 1
	}
	return fmt.Sprintf("(%d %d T:%d M:%.1f B:%.1f S:%d ai:%d)",
		p.GetPlayerID(), p.GetChairID(), p.GetTableID(), p.GetAllMoney(), p.GetBet(), see, ai)
}

func (p *Player) DescHand() string {
	return fmt.Sprintf("(%v %d)", p.gameData.cards.hand, p.gameData.cards.ty)
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
	p.gameData.status = status
}

func (p *Player) GetStatus() Status {
	return p.gameData.status
}

func (p *Player) IncrTimeoutCnt(timeout bool) {
	if !timeout {
		return
	}
	p.gameData.idleCount++
}

func (p *Player) ClearTimeoutCnt() {
	p.gameData.idleCount = 0
}

func (p *Player) GetTimeoutCnt() int32 {
	return p.gameData.idleCount
}

func (p *Player) SetOffline(offline bool) {
	p.gameData.isOffline = offline
}

func (p *Player) IsOffline() bool {
	return p.gameData.isOffline
}

func (p *Player) IsReady() bool {
	return p.gameData.status == StReady
}

func (p *Player) IsGaming() bool {
	return p.gameData.status == StGaming
}

func (p *Player) SetLastOp(op int32) {
	p.gameData.lastOp = op
}
func (p *Player) GetLastOp() int32 {
	return p.gameData.lastOp
}

func (p *Player) AddBet(bet float64) {
	if bet <= 0 {
		return
	}
	p.gameData.bet += bet
}
func (p *Player) GetBet() float64 {
	return p.gameData.bet
}

func (p *Player) SetSeen() {
	p.gameData.seen = true
}

func (p *Player) Seen() bool {
	return p.gameData.seen
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
	p.gameData.startMoney = p.GetAllMoney()
	if !p.UseMoney(bet) {
		return false
	}
	p.gameData.bet += bet
	p.SetStatus(StGaming)
	return true
}

func (p *Player) IncrPlayCnt() {
	p.gameData.playCount++
}

func (p *Player) SetCompareSeats(chairs []int32) {
	p.gameData.compareSeats = chairs
}

func (p *Player) GetCompareSeats() []int32 {
	return p.gameData.compareSeats
}

// Settle 结算
func (p *Player) Settle(totalBet float64) float64 {

	totalWin := totalBet
	profit := totalWin - p.gameData.bet

	log.Infof("Settle. p:%+v totalWin:%.1f profit:%.1f", p.Desc(), totalWin, profit)

	return profit
}
