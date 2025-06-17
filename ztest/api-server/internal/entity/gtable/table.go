package gtable

import (
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	gplayer2 "github.com/yola1107/kratos/v2/ztest/api-server/internal/entity/gplayer"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/model"
)

type Table struct {
	ID       int32          // 桌子ID
	Type     conf.TableType // 类型
	MaxCnt   int16          // 最大玩家数
	isClosed bool           // 是否停服
	stage    *Stage         // 阶段状态

	event ITableEvent

	// 游戏逻辑变量
	sitCnt   int16              // 入座玩家数量
	banker   int32              //
	active   int32              // 当前操作玩家
	curRound int32              // 当前回合轮数
	curBet   float64            // 当前需投注
	totalBet float64            // 桌子总投注
	mLog     *TableLog          // 桌子日志
	seats    []*gplayer2.Player // 玩家列表
	cards    *model.GameCards   // card信息
}

func NewTable(id int32, typ conf.TableType, c *conf.Room, e ITableEvent) *Table {
	t := &Table{
		ID:       id,
		Type:     typ,
		MaxCnt:   int16(c.Table.ChairNum),
		stage:    &Stage{},
		event:    e,
		sitCnt:   0,
		banker:   -1,
		active:   -1,
		curRound: 0,
		curBet:   c.Game.BaseMoney,
		totalBet: 0,
		mLog:     &TableLog{},
		seats:    make([]*gplayer2.Player, c.Table.ChairNum),
		cards:    &model.GameCards{},
	}
	t.cards.Init()
	t.mLog.init(id, c.LogCache)
	return t
}

func (t *Table) Reset() {

}

func (t *Table) Empty() bool {
	return t.sitCnt <= 0
}

func (t *Table) IsFull() bool {
	return t.sitCnt >= t.MaxCnt
}

func (t *Table) GetSitCnt() int32 {
	return int32(t.sitCnt)
}

// ThrowInto 入座
func (t *Table) ThrowInto(p *gplayer2.Player) bool {
	for k, v := range t.seats {
		if v != nil {
			continue
		}

		// 桌子信息
		t.seats[k] = p
		t.sitCnt++

		// 玩家信息
		p.Reset()
		p.SetTableID(t.ID)
		p.SetChairID(int32(k))
		p.SetStatus(gplayer2.StSit)

		// 通知客户端登录成功
		t.SendLoginRsp(p, model.SUCCESS, "")

		// 广播入座信息
		t.broadcastUserInfo(p)

		// 发送场景信息
		t.SendSceneInfo(p)

		// 检查游戏是否开始
		if t.stage.state == conf.StWait {
			t.checkReady()
		}

		// 上报桌子/玩家位置 todo

		t.mLog.userEnter(p, t.sitCnt)
		log.Infof("EnterTable. p:%+v sitCnt:%d", p.Desc(), t.sitCnt)
		return true
	}
	return false
}

// ThrowOff 出座
func (t *Table) ThrowOff(p *gplayer2.Player, isSwitchTable bool) bool {
	if p == nil {
		return false
	}

	if !t.CanExit(p) {
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

	// 广播玩家离桌
	t.broadcastUserQuitPush(p, isSwitchTable)

	// 重置玩家信息
	p.Reset()
	p.SetChairID(-1)
	p.SetTableID(-1)

	// 上报桌子/玩家位置 todo
	t.mLog.userExit(p, t.sitCnt, isSwitchTable)
	log.Infof("ExitTable. p:%+v sitCnt:%d isSwitch:%+v", p.Desc(), t.sitCnt, isSwitchTable)
	return true
}

// ReEnter 重进游戏
func (t *Table) ReEnter(p *gplayer2.Player) {
	// 通知客户端登录成功
	t.SendLoginRsp(p, model.SUCCESS, "ReEnter")

	// 广播入座信息
	t.broadcastUserInfo(p)

	// 发送场景信息
	t.SendSceneInfo(p)

	p.SetOffline(false)

	t.broadcastUserOffline(p)

	t.mLog.userEnter(p, t.sitCnt)
	log.Infof("ReEnterTable. p:%+v sitCnt:%d", p.Desc(), t.sitCnt)
}

// LastPlayer 上一家
func (t *Table) LastPlayer(chair int32) *gplayer2.Player {
	maxCnt := int32(t.MaxCnt)
	for i := int32(0); i < maxCnt; i++ {
		chair--
		if chair < 0 {
			chair = maxCnt - 1
		}
		if t.seats[chair] == nil || !t.seats[chair].IsGaming() {
			continue
		}
		return t.seats[chair]
	}
	return nil
}

// NextPlayer 轮流寻找玩家
func (t *Table) NextPlayer(chair int32) *gplayer2.Player {
	maxCnt := int32(t.MaxCnt)
	for i := int32(0); i < maxCnt; i++ {
		chair = (chair + 1) % maxCnt
		if t.seats[chair] == nil || !t.seats[chair].IsGaming() {
			continue
		}
		return t.seats[chair]
	}

	return nil
}

// RangePlayer 遍历玩家
func (t *Table) RangePlayer(cb func(k int32, p *gplayer2.Player) bool) {
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

func (t *Table) GetActivePlayer() *gplayer2.Player {
	active := t.active
	if active < 0 || active >= int32(t.MaxCnt) {
		return nil
	}
	return t.seats[active]
}

func (t *Table) GetNextActivePlayer() *gplayer2.Player {
	if t.active < 0 || t.active >= int32(t.MaxCnt) {
		return nil
	}
	return t.NextPlayer(t.active)
}

func (t *Table) GetPlayerByChair(chair int32) *gplayer2.Player {
	if chair < 0 || chair >= int32(t.MaxCnt) {
		return nil
	}
	return t.seats[chair]
}

func (t *Table) CanEnter(p *gplayer2.Player) bool {
	return true
}

func (t *Table) CanExit(p *gplayer2.Player) bool {
	return !p.IsGaming()
}

func (t *Table) CanSwitchTable(p *gplayer2.Player) bool {
	if p == nil {
		return false
	}
	if p.IsGaming() {
		return false
	}
	return true
}
