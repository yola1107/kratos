package table

import (
	"github.com/yola1107/kratos/v2/errors"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/model"
)

type Manager struct {
	tableList []*Table
	tableMap  map[int32]*Table
}

func NewManager(c *conf.Room, event ITableRepo) *Manager {
	tc := c.Table
	mgr := &Manager{
		tableList: make([]*Table, tc.TableNum),
		tableMap:  make(map[int32]*Table),
	}
	for i := int32(1); i <= tc.TableNum; i++ {
		tb := NewTable(i, conf.Normal, c, event)
		mgr.tableMap[i] = tb
		mgr.tableList[i-1] = tb
	}
	return mgr
}

func (m *Manager) GetTable(id int32) *Table {
	return m.tableMap[id]
}

func (m *Manager) SwitchTable(p *player.Player, c *conf.Room_Game) (err *errors.Error) {
	if p == nil {
		return model.ErrPlayerNotFound
	}

	if err := checkRoomLimit(p, c); err != nil {
		return err
	}

	oldTable := m.tableMap[p.GetTableID()]
	if oldTable == nil {
		return model.ErrTableNotFound
	}

	if !oldTable.CanSwitchTable(p) {
		return model.ErrSwitchTable
	}

	newTable := m.getTopTable(p, true)
	if newTable == nil {
		return model.ErrNotEnoughTable
	}

	if !oldTable.ThrowOff(p, true) {
		return model.ErrExitTableFail
	}

	if !newTable.ThrowInto(p) {
		return model.ErrEnterTableFail
	}

	return nil
}

func (m *Manager) ThrowInto(p *player.Player) bool {
	if p == nil {
		return false
	}
	best := m.getTopTable(p, false)
	if best == nil {
		return false
	}
	return best.ThrowInto(p)
}

func (m *Manager) getTopTable(p *player.Player, isSwitchTable bool) *Table {

	var best *Table
	var oldTableID = p.GetTableID()

	for _, t := range m.tableList {
		if t == nil {
			continue
		}
		if t.IsFull() {
			continue
		}
		if !t.CanEnter(p) {
			continue
		}
		if isSwitchTable && t.ID == oldTableID {
			continue
		}
		if best != nil && t.GetSitCnt() <= best.GetSitCnt() {
			continue
		}
		best = t
	}

	if best == nil {
		log.Warn("无可用桌子，玩家ID: %d", p.GetPlayerID())
		return nil
	}

	return best
}

// CanEnterRoom 检查是否能进房
func (m *Manager) CanEnterRoom(p *player.Player, token string, c *conf.Room_Game) (err *errors.Error) {
	if p == nil {
		return model.ErrPlayerNotFound
	}

	// 校验token
	if token == "" {
		return model.ErrTokenFail
	}

	// room limit
	if err = checkRoomLimit(p, c); err != nil {
		return err
	}
	return nil
}

func checkRoomLimit(p *player.Player, c *conf.Room_Game) (err *errors.Error) {
	money := p.GetMoney()
	vip := p.GetVipGrade()
	if money < c.MinMoney {
		return model.ErrMoneyBelowMinLimit
	}
	if money > c.MaxMoney && c.MaxMoney != -1 {
		return model.ErrMoneyOverMaxLimit
	}
	if money < c.BaseMoney {
		return model.ErrMoneyBelowBaseLimit
	}
	if vip < c.VipLimit {
		return model.ErrVipLimit
	}
	return nil
}
