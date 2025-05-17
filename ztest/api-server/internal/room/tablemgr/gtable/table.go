package gtable

import (
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/model"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/playermgr/gplayer"
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

	// 游戏逻辑变量
	nTimeOut int64 //超时
	nStage   int32 //阶段

	activeChair int32           //当前操作玩家
	cardObj     model.GameCards //牌信息
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

	//log.Warn("玩家断线重进游戏 pid:%d chair%d sn:%d ", p.GetPlayerID(), p.GetChairID(), tb.ID)
	//
	//// 通知客户端登录成功
	//p.SendEnterGameNotify(model.SUCCESS, "")
	//
	//// 广播入座信息
	//tb.SendUserInfo(p)
	//
	//// 发送场景信息
	//tb.mLogic.OnScene(p, false)
	//
	//tb.mLogic.onUserReEnterEvent(p)
	//
	//tb.mLogic.mLog.ReEnter(p)
	//
	////推送道具列表
	//tb.mLogic.OnEmojiConfigPush(p)
	//
	//if config.IsTypeScore() {
	//	tb.mLogic.UserReEnterEvent_Score(p)
	//}
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
