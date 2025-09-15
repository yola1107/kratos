package player

import (
	"fmt"
	"time"

	"github.com/yola1107/kratos/v2/library/xgo"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/model"
)

const (
	_DiceNot6MaxCount = 5 // 前 n 次必须至少出过一次6
)

// 玩家状态枚举
const (
	StFree     Status = iota // 空闲
	StSit                    // 入座
	StReady                  // 准备
	StGaming                 // 游戏中
	StGameFold               // 弃权（暂未使用）
	StGameLost               // 失败（暂未使用）
)

// Status 表示玩家当前的状态
type Status int32

// String 返回状态的字符串表示
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

// DiceSlot 表示一个骰子及其是否已被使用
type DiceSlot struct {
	Value int32 // 骰子点数
	Used  bool  // 是否已使用
}

// GameData 存储玩家在对局中的动态信息
type GameData struct {
	TableID    int32   // 所在桌号
	ChairID    int32   // 座位号
	status     Status  // 玩家状态
	startMoney float64 // 回合开始时的金币
	idleCount  int32   // 超时次数
	isOffline  bool    // 是否离线

	bet           float64           // 当前投注
	color         int32             // 颜色：0红 1黄 2绿 3蓝
	diceRollCount int               // 掷骰次数
	hasGotSix     bool              // 是否投出过6
	dices         []DiceSlot        // 当前轮骰子
	lastDices     []DiceSlot        // 上轮骰子
	paths         *model.TagRetData // 当前可行路径缓存
	finish        bool              // 是否所有棋子进入终点
	finishAt      int64             // 所有棋子到达终点时间
	pieceIds      []int32           // 当前持有的棋子ID列表
}

// Reset 清除玩家的游戏状态（不清除座位与桌号）
func (p *Player) Reset() {
	p.gameData.status = StFree
	p.gameData.startMoney = 0
	p.gameData.idleCount = 0
	p.gameData.isOffline = false
	p.gameData.bet = 0
	p.gameData.color = -1
	p.gameData.diceRollCount = 0
	p.gameData.hasGotSix = false
	p.gameData.dices = nil
	p.gameData.lastDices = nil
	p.gameData.paths = nil
	p.gameData.finish = false
	p.gameData.finishAt = 0
	p.gameData.pieceIds = nil
}

// ExitReset 玩家离桌时调用，清除座位与桌号信息
func (p *Player) ExitReset() {
	p.Reset()
	p.gameData.ChairID = -1
	p.gameData.TableID = -1
	p.gameData.status = -1
}

// Desc 打印当前玩家的关键游戏状态（调试用）
func (p *Player) Desc() string {
	return fmt.Sprintf("(%d %d T:%d St:%s ai:%d co:%v ids=%v dices:%v)",
		p.GetPlayerID(), p.GetChairID(), p.GetTableID(), p.GetStatus().String(), bool2Int(p.isRobot),
		p.GetColor(), xgo.ToJSON(p.GetPieces()), xgo.ToJSON(p.DiceListInt32()))
}

func (p *Player) DescPath() string {
	if r := p.GetPaths(); r != nil {
		return r.Desc()
	}
	return ""
}

func bool2Int(v bool) int {
	if v {
		return 1
	}
	return 0
}

// ---- Game Data Accessors ----

func (p *Player) SetTableID(tableID int32) {
	p.gameData.TableID = tableID
}

func (p *Player) GetTableID() int32 {
	return p.gameData.TableID
}

func (p *Player) SetChairID(chairID int32) {
	p.gameData.ChairID = chairID
}

func (p *Player) GetChairID() int32 {
	return p.gameData.ChairID
}

func (p *Player) IncrTimeoutCnt(timeout bool) {
	if timeout {
		p.gameData.idleCount++
	}
}

func (p *Player) ClearTimeoutCnt() {
	p.gameData.idleCount = 0
}

func (p *Player) GetTimeoutCnt() int32 {
	return p.gameData.idleCount
}

func (p *Player) SetOffline(v bool) {
	p.gameData.isOffline = v
}

func (p *Player) IsOffline() bool {
	return p.gameData.isOffline
}

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

func (p *Player) IntoGaming(bet float64) bool {
	p.gameData.startMoney = p.GetAllMoney()
	if !p.UseMoney(bet) {
		return false
	}
	p.gameData.bet += bet
	return true
}

// Settle 游戏结算，返回本局盈亏（不处理实际发放）
func (p *Player) Settle(totalBet float64) float64 {
	win := totalBet
	bet := p.gameData.bet
	profit := win - bet
	log.Debugf("Settle. p:%+v Win(%.1f) bet(%.1f) profit(%.1f)", p.Desc(), win, bet, profit)
	return profit
}

func (p *Player) AddBet(bet float64) {
	if bet > 0 {
		p.gameData.bet += bet
	}
}
func (p *Player) GetBet() float64 {
	return p.gameData.bet
}

func (p *Player) SetColor(color int32) {
	p.gameData.color = color
}

func (p *Player) GetColor() int32 {
	return p.gameData.color
}

func (p *Player) SetPieces(pieces []int32) {
	p.gameData.pieceIds = pieces
}

func (p *Player) GetPieces() []int32 {
	return p.gameData.pieceIds
}

func (p *Player) MarkPieceArrived(pieceId int32) {
	for k, id := range p.GetPieces() {
		if pieceId == id {
			p.gameData.pieceIds[k] = -id
			return
		}
	}
}
func (p *Player) SetPaths(paths *model.TagRetData) {
	p.gameData.paths = paths
}

func (p *Player) GetPaths() *model.TagRetData {
	return p.gameData.paths
}

// AddDice 添加一个骰子（未使用状态）
func (p *Player) AddDice(value int32) {
	p.gameData.dices = append(p.gameData.dices, DiceSlot{Value: value, Used: false})

	// 更新骰子次数 及 6点出现的情况
	p.gameData.diceRollCount++
	if value == 6 {
		p.gameData.hasGotSix = true
	}
}

// GetDiceSlot 返回当前骰子列表（含已用/未用）
func (p *Player) GetDiceSlot() []DiceSlot { return p.gameData.dices }

// GetLastDiceSlot 返回上轮骰子记录
func (p *Player) GetLastDiceSlot() []DiceSlot { return p.gameData.lastDices }

// UnusedDice 返回当前未被使用的骰子点数
func (p *Player) UnusedDice() []int32 {
	var out []int32
	for _, d := range p.gameData.dices {
		if !d.Used {
			out = append(out, d.Value)
		}
	}
	return out
}

// HasUnusedDice 检查是否存在某个未用骰子
func (p *Player) HasUnusedDice(value int32) bool {
	for _, d := range p.gameData.dices {
		if d.Value == value && !d.Used {
			return true
		}
	}
	return false
}

// UseDice 标记某个骰子为已使用
func (p *Player) UseDice(value int32) bool {
	for i, d := range p.gameData.dices {
		if d.Value == value && !d.Used {
			p.gameData.dices[i].Used = true
			return true
		}
	}
	return false
}

// DiceListInt32 返回当前骰子列表，使用负数表示已使用
func (p *Player) DiceListInt32() []int32 {
	dices := make([]int32, 0, len(p.gameData.dices))
	for _, d := range p.gameData.dices {
		val := d.Value
		if d.Used {
			val = -val
		}
		dices = append(dices, val)
	}
	return dices
}

// IsTripleSix 检测是否连续投出三个未使用的6（跳过回合）
func (p *Player) IsTripleSix() bool {
	count := 0
	dices := p.gameData.dices
	for i := len(dices) - 1; i >= 0 && count < 3; i-- {
		if dices[i].Used {
			continue
		}
		if dices[i].Value == 6 {
			count++
		} else {
			break
		}
	}
	return count >= 3
}

// FinishTurn 回合结束 = 所有骰子都已用完  || 所有剩余骰子不能再移动任何棋子 || 666连续3个6跳过回合
// FinishTurn 回合结束，记录当前骰子为 lastDices 并清空 dices
func (p *Player) FinishTurn() {
	p.gameData.lastDices = make([]DiceSlot, len(p.gameData.dices))
	copy(p.gameData.lastDices, p.gameData.dices)
	p.gameData.dices = nil
}

func (p *Player) SetFinish() {
	p.gameData.finish = true
	p.gameData.finishAt = time.Now().Unix()
}

func (p *Player) GetFinishAt() int64 {
	return p.gameData.finishAt
}

func (p *Player) IsFinish() bool {
	return p.gameData.finish
}
