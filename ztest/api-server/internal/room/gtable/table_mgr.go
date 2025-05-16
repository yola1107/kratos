package gtable

import (
	"time"

	"github.com/yola1107/kratos/v2/library/gtimer"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gplayer"
)

type Manager struct {
	tableList []*Table
	tableMap  map[int32]*Table
	closed    bool
}

func NewManager() *Manager {
	c := conf.GetTC()
	mgr := &Manager{
		tableList: make([]*Table, c.TableNum),
		tableMap:  make(map[int32]*Table),
	}
	for i := int32(1); i <= c.TableNum; i++ {
		tb := &Table{ID: i, MaxCnt: int16(c.ChairNum)}
		tb.Init()
		mgr.tableMap[i] = tb
		mgr.tableList[i-1] = tb
	}
	//log.Infof("tableMgr init. tables=%d chairs=%d", c.TableNum, c.ChairNum)
	return mgr
}

func (m *Manager) Start() {
	gtimer.Forever(nil, time.Second/2, m.onTimer)
}

func (m *Manager) Close() {
	m.closed = true
}

func (m *Manager) onTimer() {
	if m.closed {
		return
	}
	for _, t := range m.tableList {
		if t.IsRunning() {
			t.OnTimer()
		}
	}
}

func (m *Manager) GetTable(id int32) *Table {
	return m.tableMap[id]
}

func (m *Manager) ThrowInto(p *gplayer.Player) bool {
	best := m.getTopTable(p, false)
	if best == nil {
		return false
	}
	return best.ThrowInto(p, false)
}

func (m *Manager) getTopTable(p *gplayer.Player, canSwitch bool) *Table {
	var best, old *Table
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
		log.Warn("无可用桌子，玩家ID: %d", p.GetUID())
	}
	return best
}

func (m *Manager) SwitchTable(p *gplayer.Player) bool {
	return false
}
