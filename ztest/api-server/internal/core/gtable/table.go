package gtable

import (
	"time"

	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/glog"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/gplayer"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/model"
)

const (
	TableStateStopped = 0
	TableStateRunning = 1
)

type Table struct {
	ID     int32 // 桌子ID
	MaxCnt int16 // 最大玩家数

	// old 可以去除
	uiGameState uint16 // 游戏状态

	// 游戏逻辑变量
	stage         int32         // 阶段
	lastStage     int32         // 上一阶段
	stageStart    time.Time     // 阶段开始时间
	stageDuration time.Duration // 阶段持续时间
	stageTimerID  int64         // 阶段定时器ID

	sitCnt      int16             // 座位上的玩家
	activeChair int32             // 当前操作玩家
	chairList   []*gplayer.Player // 玩家列表
	gameCards   model.GameCards   // card信息
	tableLogger glog.TableLog     // 桌子日志

	isClosed bool // 是否停服
}

func (tb *Table) Init() {
	tb.sitCnt = 0
	tb.chairList = make([]*gplayer.Player, tb.MaxCnt)
	tb.uiGameState = TableStateStopped
}

// IsFull full
func (tb *Table) IsFull() bool {
	return tb.sitCnt >= tb.MaxCnt
}

func (tb *Table) GetSitCnt() int32 {
	return int32(tb.sitCnt)
}

func (tb *Table) Empty() bool {
	return tb.sitCnt <= 0
}

func (tb *Table) IsRunning() bool {
	return tb.uiGameState == TableStateRunning
}

func (tb *Table) SetClose(b bool) {
	tb.isClosed = b
}

func (tb *Table) IsClosed() bool {
	return tb.isClosed
}

// ThrowInto 入座
func (tb *Table) ThrowInto(p *gplayer.Player) bool {
	for k, v := range tb.chairList {
		if v != nil {
			continue
		}

		// 桌子信息
		tb.chairList[k] = p
		tb.sitCnt++

		// 玩家信息
		p.SetTableID(tb.ID)
		p.SetChairID(int32(k))

		// 广播入座信息
		tb.BroadcastUserEnter(p)
		tb.SendTableInfo(p)

		//
		p.Reset()

		/// 检查游戏是否开始
		if tb.canStart() {
			if tb.uiGameState == TableStateStopped {
				tb.start()
			}
			tb.uiGameState = TableStateRunning
		}
		return true
	}
	return false
}

// ThrowOff 出座
func (tb *Table) ThrowOff(p *gplayer.Player) bool {
	if p == nil {
		return false
	}

	if !tb.canExit(p) {
		return false
	}

	isFind := false
	if p.GetChairID() >= 0 {
		if p == tb.chairList[p.GetChairID()] {
			isFind = true
		}
	}

	if !isFind {
		return false
	}

	tb.chairList[p.GetChairID()] = nil
	tb.sitCnt--

	return true
}

// ReEnter 重进游戏
func (tb *Table) ReEnter(p *gplayer.Player) {
}

// LastPlayer 上一家
func (tb *Table) LastPlayer(chair int32) *gplayer.Player {
	maxCnt := tb.MaxCnt
	for {
		chair--

		if chair < 0 {
			chair = int32(tb.MaxCnt) - 1
		}

		p := tb.chairList[chair]
		if p != nil {
			return p
		}

		maxCnt--
		if maxCnt < 0 {
			return nil
		}
	}
}

// NextPlayer 轮流寻找玩家
func (tb *Table) NextPlayer(chair int32) *gplayer.Player {
	maxCnt := tb.MaxCnt
	for {
		chair++

		if chair >= int32(tb.MaxCnt) {
			chair = 0
		}

		p := tb.chairList[chair]
		if p != nil {
			return p
		}

		maxCnt--
		if maxCnt < 0 {
			return nil
		}
	}
}

// RangePlayer 遍历玩家
func (tb *Table) RangePlayer(cb func(k int32, p *gplayer.Player) bool) {
	if cb == nil {
		return
	}
	for k, p := range tb.chairList {
		if p == nil {
			continue
		}
		if !cb(int32(k), p) {
			break
		}
	}
}

func (tb *Table) GetActivePlayer() *gplayer.Player {
	active := tb.activeChair
	if active < 0 || active >= int32(tb.MaxCnt) {
		return nil
	}
	return tb.chairList[active]
}

func (tb *Table) GetNextActivePlayer() *gplayer.Player {
	if tb.activeChair < 0 || tb.activeChair >= int32(tb.MaxCnt) {
		return nil
	}
	return tb.NextPlayer(tb.activeChair)
}

func (tb *Table) GetPlayerByChair(chair int32) *gplayer.Player {
	if chair < 0 || chair >= int32(tb.MaxCnt) {
		return nil
	}
	return tb.chairList[chair]
}
