package gtable

import (
	"time"

	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/model"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gplayer"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/iface"
)

type Table struct {
	TableID  int32 // 桌子ID
	MaxCnt   int16 // 最大玩家数
	isClosed bool  // 是否停服

	stage gameStage       // 阶段状态
	repo  iface.IRoomRepo // 定时任务

	sitCnt int16             // 入座玩家数量
	active int32             // 当前操作玩家
	seats  []*gplayer.Player // 玩家列表
	cards  model.GameCards   // card信息
	logger *tableLogger      // 桌子日志

	// 游戏逻辑变量
	curRound int32   // 当前轮数
	curBet   float64 // 当前投注
	totalBet float64 // 总投注
}

type gameStage struct {
	curr      int32         // 阶段
	last      int32         // 上一阶段
	timerID   int64         // 阶段定时器ID
	startTime time.Time     // 阶段开始时间
	duration  time.Duration // 阶段持续时间
}

func (t *Table) Init() {
	t.sitCnt = 0
	t.seats = make([]*gplayer.Player, t.MaxCnt)
}

func (t *Table) Reset() {}

// IsFull full
func (t *Table) IsFull() bool {
	return t.sitCnt >= t.MaxCnt
}

func (t *Table) GetSitCnt() int32 {
	return int32(t.sitCnt)
}

func (t *Table) Empty() bool {
	return t.sitCnt <= 0
}

func (t *Table) SetClose(b bool) {
	t.isClosed = b
}

func (t *Table) IsClosed() bool {
	return t.isClosed
}

// ThrowInto 入座
func (t *Table) ThrowInto(p *gplayer.Player) bool {
	for k, v := range t.seats {
		if v != nil {
			continue
		}

		// 桌子信息
		t.seats[k] = p
		t.sitCnt++

		// 玩家信息
		p.SetTableID(t.TableID)
		p.SetChairID(int32(k))
		p.Reset()

		// 广播入座信息
		t.BroadcastUserEnter(p)
		t.SendTableInfo(p)

		// 检查游戏是否开始
		if t.stage.curr == conf.StWait {

		}

		return true
	}
	return false
}

// ThrowOff 出座
func (t *Table) ThrowOff(p *gplayer.Player) bool {
	if p == nil {
		return false
	}

	if !t.canExit(p) {
		return false
	}

	isFind := false
	if p.GetChairID() >= 0 {
		if p == t.seats[p.GetChairID()] {
			isFind = true
		}
	}

	if !isFind {
		return false
	}

	t.seats[p.GetChairID()] = nil
	t.sitCnt--

	return true
}

// ReEnter 重进游戏
func (t *Table) ReEnter(p *gplayer.Player) {
}

// LastPlayer 上一家
func (t *Table) LastPlayer(chair int32) *gplayer.Player {
	maxCnt := t.MaxCnt
	for {
		chair--

		if chair < 0 {
			chair = int32(t.MaxCnt) - 1
		}

		p := t.seats[chair]
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
func (t *Table) NextPlayer(chair int32) *gplayer.Player {
	maxCnt := t.MaxCnt
	for {
		chair++

		if chair >= int32(t.MaxCnt) {
			chair = 0
		}

		p := t.seats[chair]
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
func (t *Table) RangePlayer(cb func(k int32, p *gplayer.Player) bool) {
	if cb == nil {
		return
	}
	for k, p := range t.seats {
		if p == nil {
			continue
		}
		if !cb(int32(k), p) {
			break
		}
	}
}

func (t *Table) GetActivePlayer() *gplayer.Player {
	active := t.active
	if active < 0 || active >= int32(t.MaxCnt) {
		return nil
	}
	return t.seats[active]
}

func (t *Table) GetNextActivePlayer() *gplayer.Player {
	if t.active < 0 || t.active >= int32(t.MaxCnt) {
		return nil
	}
	return t.NextPlayer(t.active)
}

func (t *Table) GetPlayerByChair(chair int32) *gplayer.Player {
	if chair < 0 || chair >= int32(t.MaxCnt) {
		return nil
	}
	return t.seats[chair]
}
