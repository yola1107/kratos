package gtable

import (
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gplayer"
)

const (
	TableStateStopped = 0
	TableStateRunning = 1
)

type Table struct {
	ID          int32             // 桌子ID
	MaxCnt      int16             // 最大玩家数
	sitCnt      int16             // 座位上的玩家
	chairList   []*gplayer.Player // 玩家列表
	uiGameState uint16            // 游戏状态
	closed      bool              // 是否停服
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
	tb.closed = b
}

func (tb *Table) IsClosed() bool {
	return tb.closed
}

// OnTimer 桌子定时
func (tb *Table) OnTimer() {
}

// ThrowInto 入座
func (tb *Table) ThrowInto(p *gplayer.Player, CanSwitchTable bool) bool {
	for k, v := range tb.chairList {
		if v != nil {
			continue
		}

		// 桌子信息
		tb.chairList[k] = p
		tb.sitCnt++

		///// 检查游戏是否开始
		//if tb.CanStart() {
		//	if tb.uiGameState == TableStateStopped {
		//		tb.mLogic.start()
		//	}
		//	tb.uiGameState = TableStateRunning
		//}
		return true
	}
	return false
}

// ThrowOff 出座
func (tb *Table) ThrowOff(p *gplayer.Player) bool {
	if p == nil {
		return false
	}

	//isFind := false
	////if p.GetChairID() >= 0 {
	////	if p == tb.chairList[player.GetChairID()] {
	////		tb.chairList[player.GetChairID()] = nil
	////		isFind = true
	////	}
	////}
	//
	//if !isFind {
	//	return false
	//}

	tb.sitCnt--

	return true
}
