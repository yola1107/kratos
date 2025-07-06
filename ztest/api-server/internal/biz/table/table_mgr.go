package table

import (
	"sync"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/pkg/codes"
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
	tc := c.Table
	mgr := &Manager{
		repo: repo,
	}
	for i := int32(1); i <= tc.TableNum; i++ {
		tb := NewTable(i, Normal, c, repo)
		mgr.tableMap.Store(tb.ID, tb)
	}
	return mgr
}

func (m *Manager) Start() error {
	return nil
}

func (m *Manager) Close() {
	return
}

func (m *Manager) GetTableList() []*Table {
	return m.GetTableListWith(All)
}

func (m *Manager) GetTableListWith(kinds KindTableList) []*Table {
	tc := m.repo.GetRoomConfig().GetTable()

	tableList := make([]*Table, 0)
	for i := int32(1); i <= tc.TableNum; i++ {
		t := m.GetTable(i)
		if t == nil {
			continue
		}
		if kinds == NoEmpty && !t.Empty() {
			tableList = append(tableList, t)
		}
		if kinds == NoFull && !t.IsFull() {
			tableList = append(tableList, t)
		}
		if kinds == All {
			tableList = append(tableList, t)
		}
	}
	return tableList
}

// GetTable 根据桌子ID获取桌子
func (m *Manager) GetTable(id int32) *Table {
	if table, ok := m.tableMap.Load(id); ok {
		return table.(*Table)
	}
	return nil
}

// SwitchTable 玩家请求换桌
func (m *Manager) SwitchTable(p *player.Player, gameConf *conf.Room_Game) (int32, string) {
	if p == nil {
		return codes.PLAYER_NOT_FOUND, ""
	}

	if code, msg := CheckRoomLimit(p, gameConf); code != 0 {
		return code, msg
	}

	oldTable := m.GetTable(p.GetTableID())
	if oldTable == nil {
		return codes.TABLE_NOT_FOUND, "TABLE_NOT_FOUND"
	}

	if !oldTable.CanSwitchTable(p) {
		return codes.SWITCH_TABLE, "SWITCH_TABLE"
	}

	newTable := m.selectBestTable(p, true)
	if newTable == nil {
		return codes.NOT_ENOUGH_TABLE, "NOT_ENOUGH_TABLE"
	}

	if !oldTable.ThrowOff(p, true) {
		return codes.EXIT_TABLE_FAIL, "EXIT_TABLE_FAIL"
	}

	if !newTable.ThrowInto(p) {
		return codes.ENTER_TABLE_FAIL, "ENTER_TABLE_FAIL"
	}

	return 0, ""
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

	notFull := m.GetTableListWith(NoFull)
	for _, t := range notFull {
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
func (m *Manager) CanEnterRoom(p *player.Player, token string, gameConf *conf.Room_Game) (int32, string) {
	if p == nil {
		return codes.PLAYER_NOT_FOUND, "PLAYER_NOT_FOUND"
	}

	if token == "" {
		return codes.TOKEN_FAIL, "TOKEN_FAIL"
	}

	return CheckRoomLimit(p, gameConf)
}

// CheckRoomLimit 校验玩家的金币、VIP等级是否符合房间限制
func CheckRoomLimit(p *player.Player, gameConf *conf.Room_Game) (int32, string) {
	money := p.GetAllMoney()
	vip := p.GetVipGrade()

	if money < gameConf.MinMoney {
		return codes.MONEY_BELOW_MIN_LIMIT, "MONEY_BELOW_MIN_LIMIT"
	}
	if gameConf.MaxMoney != -1 && money > gameConf.MaxMoney {
		return codes.MONEY_OVER_MAX_LIMIT, "MONEY_OVER_MAX_LIMIT"
	}
	if money < gameConf.BaseMoney {
		return codes.MONEY_BELOW_BASE_LIMIT, "MONEY_BELOW_BASE_LIMIT"
	}
	if vip < gameConf.VipLimit {
		return codes.VIP_LIMIT, "VIP_LIMIT"
	}
	return codes.SUCCESS, ""
}
