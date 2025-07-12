package player

import (
	"fmt"

	"github.com/yola1107/kratos/v2/library/ext"
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

func (s Status) String() string {
	switch s {
	case StFree:
		return "Free"
	case StSit:
		return "Sit"
	case StReady:
		return "Ready"
	case StGaming:
		return "Gaming"
	case StGameFold:
		return "Fold"
	case StGameLost:
		return "Lost"
	default:
		return fmt.Sprintf("%d", s)
	}
}

type GameData struct {
	TableID    int32   // 桌子ID
	ChairID    int32   // 椅子ID
	status     Status  // 0 StFree 1 StSit 2 StReady 3 StGaming
	startMoney float64 // 局数开始时的金币
	idleCount  int32   // 超时/托管次数
	isOffline  bool    // 是否离线
	bet        float64 // 投注
	lastOp     int32   // 上一次操作
	cards      []int32 // 手牌

}

func (p *Player) Reset() {
	p.gameData.status = 0
	p.gameData.startMoney = 0
	p.gameData.idleCount = 0
	p.gameData.isOffline = false
	p.gameData.bet = 0
	p.gameData.lastOp = 0
	p.gameData.cards = nil
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
	bool2Int := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	return fmt.Sprintf("(%d %d T:%d M:%.1f B:%.1f St:%v ai:%d offline:%v)", p.GetPlayerID(), p.GetChairID(), p.GetTableID(),
		p.GetAllMoney(), p.GetBet(), p.GetStatus(), bool2Int(p.isRobot), bool2Int(p.IsOffline()))
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

// func (p *Player) SetStatus(status Status) {
// 	p.gameData.status = status
// }

func (p *Player) GetStatus() Status {
	return p.gameData.status
}

func (p *Player) SetSit() {
	p.gameData.status = StSit
}

func (p *Player) SetReady() {
	p.gameData.status = StReady
}

func (p *Player) IsReady() bool {
	return p.gameData.status == StReady
}

func (p *Player) SetGaming() {
	p.gameData.status = StGaming
}

func (p *Player) IsGaming() bool {
	return p.gameData.status == StGaming
}

func (p *Player) SetFold() {
	p.gameData.status = StGameFold
}

func (p *Player) IsFold() bool {
	return p.gameData.status == StGameFold
}

func (p *Player) SetLost() {
	p.gameData.status = StGameFold
}

func (p *Player) IsLost() bool {
	return p.gameData.status == StGameLost
}

// func (p *Player) SetLastOp(op v1.ACTION) {
// 	p.gameData.lastOp = op
// }
// func (p *Player) GetLastOp() v1.ACTION {
// 	return p.gameData.lastOp
// }

func (p *Player) AddBet(bet float64) {
	if bet <= 0 {
		return
	}
	p.gameData.bet += bet
}
func (p *Player) GetBet() float64 {
	return p.gameData.bet
}

func (p *Player) GetCards() []int32 {
	return p.gameData.cards
}

func (p *Player) AddCards(cs []int32) {
	p.gameData.cards = append(p.gameData.cards, cs...)
}

func (p *Player) RemoveCard(card int32) {
	p.gameData.cards = ext.SliceDel(p.gameData.cards, card)
}

func (p *Player) IntoGaming(bet float64) bool {
	p.gameData.startMoney = p.GetAllMoney()
	if !p.UseMoney(bet) {
		return false
	}
	p.gameData.bet += bet
	return true
}

// Settle 结算
func (p *Player) Settle(totalBet float64) float64 {

	win := totalBet
	bet := p.gameData.bet
	profit := win - bet

	log.Debugf("Settle. p:%+v Win(%.1f) bet(%.1f) profit(%.1f)", p.Desc(), win, bet, profit)

	return profit
}
