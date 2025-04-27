package gtable

import (
	"time"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gplayer"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/iface"
)

type TableManager struct {
	tableList []*Table
	tableMap  map[int32]*Table
	repo      iface.IRoomRepo
}

func NewTableManager(c *conf.Room, repo iface.IRoomRepo) *TableManager {
	tc := c.GetTable()
	mgr := &TableManager{
		tableList: make([]*Table, tc.TableNum),
		tableMap:  make(map[int32]*Table),
		repo:      repo,
	}
	for i := int32(1); i <= tc.TableNum; i++ {
		tb := &Table{
			TableID: i,
			MaxCnt:  int16(c.Table.ChairNum),
		}
		mgr.tableMap[i] = tb
		mgr.tableList[i-1] = tb
	}
	// log.Infof("tableMgr init. tables=%d chairs=%d", tc.TableNum, tc.ChairNum)
	return mgr
}

func (m *TableManager) Start() error {
	// log.Infof("TableMgr start")
	m.repo.GetTimer().Once(5*time.Second, func() {
		log.Infof("im back")
	})
	// gtimer.Forever(nil, time.Second/2, m.onTimer)
	return nil
}

func (m *TableManager) Close() {
}

func (m *TableManager) onTimer() {
}

func (m *TableManager) GetTable(id int32) *Table {
	return m.tableMap[id]
}

func (m *TableManager) ThrowInto(p *gplayer.Player) bool {
	if p == nil {
		return false
	}
	best := m.getTopTable(p, false)
	if best == nil {
		return false
	}
	return best.ThrowInto(p)
}

func (m *TableManager) getTopTable(p *gplayer.Player, canSwitch bool) *Table {
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
		log.Warn("无可用桌子，玩家ID: %d", p.GetPlayerID())
	}
	return best
}

func (m *TableManager) SwitchTable(p *gplayer.Player) bool {
	return false
}
