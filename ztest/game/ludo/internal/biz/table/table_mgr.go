package table

import (
	"sync"

	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/pkg/codes"
)

type KindTableList int32

const (
	All KindTableList = iota
	NoEmpty
	NoFull
)

type Manager struct {
	repo     Repo
	tableMap sync.Map // map[int32]*Table
}

func NewManager(c *conf.Room, repo Repo) *Manager {
	m := &Manager{repo: repo}
	for i := int32(1); i <= c.Table.TableNum; i++ {
		m.tableMap.Store(i, NewTable(i, Normal, c, repo))
	}
	return m
}

func (m *Manager) Start() error { return nil }
func (m *Manager) Close()       {}

func (m *Manager) GetTable(id int32) *Table {
	if v, ok := m.tableMap.Load(id); ok {
		return v.(*Table)
	}
	return nil
}

func (m *Manager) GetTableList() []*Table {
	return m.GetTableListWith(All)
}

func (m *Manager) GetTableListWith(kind KindTableList) []*Table {
	tc := m.repo.GetRoomConfig().GetTable()
	tables := make([]*Table, 0, tc.TableNum)
	for i := int32(1); i <= tc.TableNum; i++ {
		t := m.GetTable(i)
		if t == nil {
			continue
		}
		switch kind {
		case NoEmpty:
			if !t.Empty() {
				tables = append(tables, t)
			}
		case NoFull:
			if !t.IsFull() {
				tables = append(tables, t)
			}
		case All:
			tables = append(tables, t)
		}
	}
	return tables
}

// ThrowInto 尝试将玩家放入合适桌子
func (m *Manager) ThrowInto(p *player.Player) (int32, string) {
	if p == nil {
		return codes.PLAYER_INVALID, "PLAYER_INVALID"
	}
	if m.tryFindAndEnter(p, false, true) ||
		m.tryFindAndEnter(p, false, false) {
		return codes.SUCCESS, ""
	}
	return codes.NOT_ENOUGH_TABLE, "NOT_ENOUGH_TABLE"
}

// SwitchTable 玩家请求换桌
func (m *Manager) SwitchTable(p *player.Player, conf *conf.Room_Game) (int32, string) {
	if p == nil {
		return codes.PLAYER_INVALID, "PLAYER_INVALID"
	}

	if code, msg := CheckRoomLimit(p, conf); code != codes.SUCCESS {
		return code, msg
	}

	old := m.GetTable(p.GetTableID())
	if old == nil {
		return codes.TABLE_NOT_FOUND, "TABLE_NOT_FOUND"
	}
	if !old.CanSwitchTable(p) {
		return codes.SWITCH_TABLE, "SWITCH_TABLE"
	}
	if !old.ThrowOff(p, true) {
		return codes.EXIT_TABLE_FAIL, "EXIT_TABLE_FAIL"
	}

	if m.tryFindAndEnter(p, true, true) ||
		m.tryFindAndEnter(p, true, false) {
		return codes.SUCCESS, ""
	}

	return codes.ENTER_TABLE_FAIL, "ENTER_TABLE_FAIL"
}

// CanEnterRoom 判断玩家是否满足进入房间条件
func (m *Manager) CanEnterRoom(p *player.Player, token string, conf *conf.Room_Game) (int32, string) {
	if p == nil {
		return codes.PLAYER_NOT_FOUND, "PLAYER_NOT_FOUND"
	}
	if token == "" {
		return codes.TOKEN_FAIL, "TOKEN_FAIL"
	}
	return CheckRoomLimit(p, conf)
}

// CheckRoomLimit 校验金币/VIP是否符合房间限制
func CheckRoomLimit(p *player.Player, conf *conf.Room_Game) (int32, string) {
	money, vip := p.GetAllMoney(), p.GetVipGrade()
	if money < conf.MinMoney {
		return codes.MONEY_BELOW_MIN_LIMIT, "MONEY_BELOW_MIN_LIMIT"
	}
	if conf.MaxMoney != -1 && money > conf.MaxMoney {
		return codes.MONEY_OVER_MAX_LIMIT, "MONEY_OVER_MAX_LIMIT"
	}
	if money < conf.BaseMoney {
		return codes.MONEY_BELOW_BASE_LIMIT, "MONEY_BELOW_BASE_LIMIT"
	}
	if vip < conf.VipLimit {
		return codes.VIP_LIMIT, "VIP_LIMIT"
	}
	return codes.SUCCESS, ""
}

func (m *Manager) tryFindAndEnter(p *player.Player, isSwitch, preferFew bool) bool {
	oldID := p.GetTableID()
	tc := m.repo.GetRoomConfig().GetTable()
	for i := int32(1); i <= tc.TableNum; i++ {
		t := m.GetTable(i)
		if t == nil || t.IsFull() || !t.CanEnter(p) || (isSwitch && t.ID == oldID) {
			continue
		}
		if preferFew && t.sitCnt > 1 {
			continue
		}
		if t.ThrowInto(p) {
			return true
		}
	}
	return false
}

// // 遍历桌子尝试进入，成功返回 true
// func (m *Manager) tryFindAndEnter(p *player.Player, isSwitch, preferFew bool) bool {
// 	oldID := p.GetTableID()
// 	found := false
// 	m.rangeTables(func(t *Table) {
// 		if found || t.IsFull() || !t.CanEnter(p) || (isSwitch && t.ID == oldID) {
// 			return
// 		}
// 		if preferFew && t.sitCnt > 1 {
// 			return
// 		}
// 		if t.ThrowInto(p) {
// 			found = true
// 		}
// 	})
// 	return found
// }
//
// // 遍历桌子执行操作
// func (m *Manager) rangeTables(f func(t *Table)) {
// 	tc := m.repo.GetRoomConfig().GetTable()
// 	for i := int32(1); i <= tc.TableNum; i++ {
// 		if t := m.GetTable(i); t != nil {
// 			f(t)
// 		}
// 	}
// }
