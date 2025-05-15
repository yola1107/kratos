package tablemgr

import (
	"time"

	"github.com/yola1107/kratos/v2/library/gtimer"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/player"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/table"
)

type TableMgr struct {
	tableList []*table.Table
	tableMap  map[int32]*table.Table
	closed    bool
}

func New() *TableMgr {
	c := conf.GetTC()
	mgr := &TableMgr{
		tableList: make([]*table.Table, c.TableNum),
		tableMap:  make(map[int32]*table.Table),
	}
	for i := int32(1); i <= c.TableNum; i++ {
		tb := &table.Table{ID: i, MaxCnt: int16(c.ChairNum)}
		tb.Init()
		mgr.tableList[i-1] = tb
		mgr.tableMap[i] = tb
	}
	log.Infof("TableMgr init. tables=%d chairs=%d", c.TableNum, c.ChairNum)
	return mgr
}

func (m *TableMgr) Start() {
	gtimer.Forever(nil, time.Second/2, m.onTimer)
}

func (m *TableMgr) Stop() {
	m.closed = true
	// TODO: 关闭所有桌子逻辑
}

func (m *TableMgr) onTimer() {
	if m.closed {
		return
	}
	for _, t := range m.tableList {
		if t.IsRunning() {
			t.OnTimer()
		}
	}
}

func (m *TableMgr) GetTable(id int32) *table.Table {
	return m.tableMap[id]
}

func (m *TableMgr) ThrowInto(p *player.Player) bool {
	best := m.getTopTable(p, false)
	if best == nil {
		return false
	}
	return best.ThrowInto(p, false)
}

func (m *TableMgr) getTopTable(p *player.Player, canSwitch bool) *table.Table {
	var best, old *table.Table
	if canSwitch {
		old = m.GetTable(p.GetTableID())
	}
	for _, t := range m.tableList {
		if t == nil || t == old || t.IsFull() {
			continue
		}
		if best != nil && t.GetSitCnt() <= best.GetSitCnt() {
			continue
		}
		best = t
	}
	if best == nil {
		log.Warn("无可用桌子，玩家ID: %d", p.GetID())
	}
	return best
}

func (m *TableMgr) SwitchTable(p *player.Player) bool {
	return false
}
