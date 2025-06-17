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
		tableMap:  make(map[int32]*Table, tc.TableNum),
	}
	for i := int32(1); i <= tc.TableNum; i++ {
		tb := NewTable(i, conf.Normal, c, event)
		mgr.tableMap[i] = tb
		mgr.tableList[i-1] = tb
	}
	return mgr
}

// GetTable 根据桌子ID获取桌子
func (m *Manager) GetTable(id int32) *Table {
	return m.tableMap[id]
}

// SwitchTable 玩家请求换桌
func (m *Manager) SwitchTable(p *player.Player, gameConf *conf.Room_Game) *errors.Error {
	if p == nil {
		return model.ErrPlayerNotFound
	}

	if err := checkRoomLimit(p, gameConf); err != nil {
		return err
	}

	oldTable := m.tableMap[p.GetTableID()]
	if oldTable == nil {
		return model.ErrTableNotFound
	}

	if !oldTable.CanSwitchTable(p) {
		return model.ErrSwitchTable
	}

	newTable := m.selectBestTable(p, true)
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

// ThrowInto 尝试将玩家放入合适桌子
func (m *Manager) ThrowInto(p *player.Player) bool {
	if p == nil {
		return false
	}

	bestTable := m.selectBestTable(p, false)
	if bestTable == nil {
		return false
	}

	return bestTable.ThrowInto(p)
}

// selectBestTable 获取最合适的桌子，isSwitch表示是否为换桌请求
func (m *Manager) selectBestTable(p *player.Player, isSwitch bool) *Table {
	var best *Table
	oldTableID := p.GetTableID()

	for _, t := range m.tableList {
		if t == nil || t.IsFull() || !t.CanEnter(p) {
			continue
		}
		if isSwitch && t.ID == oldTableID {
			continue
		}
		// 选座人数多的桌子（有玩家的桌子优先）
		if best != nil && t.GetSitCnt() <= best.GetSitCnt() {
			continue
		}
		best = t
	}

	if best == nil {
		log.Warnf("No available table found for player ID: %d", p.GetPlayerID())
	}

	return best
}

// CanEnterRoom 判断玩家是否满足进入房间条件
func (m *Manager) CanEnterRoom(p *player.Player, token string, gameConf *conf.Room_Game) *errors.Error {
	if p == nil {
		return model.ErrPlayerNotFound
	}

	if token == "" {
		return model.ErrTokenFail
	}

	return checkRoomLimit(p, gameConf)
}

// checkRoomLimit 校验玩家的金币、VIP等级是否符合房间限制
func checkRoomLimit(p *player.Player, gameConf *conf.Room_Game) *errors.Error {
	money := p.GetMoney()
	vip := p.GetVipGrade()

	if money < gameConf.MinMoney {
		return model.ErrMoneyBelowMinLimit
	}
	if gameConf.MaxMoney != -1 && money > gameConf.MaxMoney {
		return model.ErrMoneyOverMaxLimit
	}
	if money < gameConf.BaseMoney {
		return model.ErrMoneyBelowBaseLimit
	}
	if vip < gameConf.VipLimit {
		return model.ErrVipLimit
	}
	return nil
}
