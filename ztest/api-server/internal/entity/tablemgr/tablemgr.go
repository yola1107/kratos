package tablemgr

import (
	"github.com/yola1107/kratos/v2/errors"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/entity/gplayer"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/entity/gtable"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/model"
)

type TableManager struct {
	tableList []*gtable.Table
	tableMap  map[int32]*gtable.Table
}

func NewTableManager(c *conf.Room, event gtable.ITableEvent) *TableManager {
	tc := c.Table
	mgr := &TableManager{
		tableList: make([]*gtable.Table, tc.TableNum),
		tableMap:  make(map[int32]*gtable.Table),
	}
	for i := int32(1); i <= tc.TableNum; i++ {
		tb := gtable.NewTable(i, conf.Normal, c, event)
		mgr.tableMap[i] = tb
		mgr.tableList[i-1] = tb
	}
	return mgr
}

func (m *TableManager) GetTable(id int32) *gtable.Table {
	return m.tableMap[id]
}

func (m *TableManager) SwitchTable(p *gplayer.Player, c *conf.Room_Game) (err *errors.Error) {
	if p == nil {
		return model.ErrPlayerNotFound
	}

	if err := checkRoomLimit(p, c); err != nil {
		return err
	}

	oldTable := m.tableMap[p.GetTableID()]
	if oldTable == nil {
		return
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

func (m *TableManager) getTopTable(p *gplayer.Player, isSwitchTable bool) *gtable.Table {

	var best *gtable.Table
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
func (m *TableManager) CanEnterRoom(p *gplayer.Player, token string, c *conf.Room_Game) (err *errors.Error) {
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

func checkRoomLimit(p *gplayer.Player, c *conf.Room_Game) (err *errors.Error) {
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

// import (
// 	"github.com/yola1107/kratos/v2/ztest/api-server/internal/entity/gtable"
// )
//
// type TableManager struct {
// 	tableList []*gtable.Table
// 	tableMap  map[int32]*gtable.Table
// }

//
// import (
// 	"time"
//
// 	"github.com/yola1107/kratos/v2/errors"
// 	"github.com/yola1107/kratos/v2/log"
// 	"github.com/yola1107/kratos/v2/ztest/api-server/internal/entity/gplayer"
// 	"github.com/yola1107/kratos/v2/ztest/api-server/internal/entity/gtable"
// 	"github.com/yola1107/kratos/v2/ztest/api-server/internal/entity/iface"
//
// 	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
// 	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
// 	"github.com/yola1107/kratos/v2/ztest/api-server/internal/model"
// )
//
// type TableManager struct {
// 	tableList []*gtable.Table
// 	tableMap  map[int32]*gtable.Table
// 	repo      iface.IRoomRepo
// }
//
// func NewTableManager(c *conf.Room, repo iface.IRoomRepo) *TableManager {
// 	tc := c.GetTable()
// 	mgr := &TableManager{
// 		tableList: make([]*gtable.Table, tc.TableNum),
// 		tableMap:  make(map[int32]*gtable.Table),
// 		repo:      repo,
// 	}
// 	for i := int32(1); i <= tc.TableNum; i++ {
// 		tb := gtable.NewTable(i, conf.Normal, c, repo)
// 		mgr.tableMap[i] = tb
// 		mgr.tableList[i-1] = tb
// 	}
// 	return mgr
// }
//
// func (m *TableManager) Start() error {
// 	m.repo.GetTimer().Once(5*time.Second, func() {
// 		log.Infof("im back")
// 	})
// 	// gtimer.Forever(nil, time.Second/2, m.onTimer)
// 	return nil
// }
//
// func (m *TableManager) Close() {
// }
//
// func (m *TableManager) onTimer() {
// }
//
// func (m *TableManager) GetTable(id int32) *gtable.Table {
// 	return m.tableMap[id]
// }
//
// func (m *TableManager) SwitchTable(p *gplayer.Player) (ok bool) {
// 	if p == nil {
// 		return
// 	}
//
// 	if err := checkRoomLimit(p, m.repo.GetRoomConfig().Game); err != nil {
// 		p.SendSwitchTableRsp(err)
// 	}
//
// 	oldTable := m.tableMap[p.GetTableID()]
// 	if oldTable == nil {
// 		return
// 	}
//
// 	if !oldTable.CanSwitchTable(p) {
// 		p.SendSwitchTableRsp(model.ErrSwitchTable)
// 		return
// 	}
//
// 	newTable := m.getTopTable(p, true)
// 	if newTable == nil {
// 		p.SendSwitchTableRsp(model.ErrNotEnoughTable)
// 		return
// 	}
//
// 	if !oldTable.ThrowOff(p, true) {
// 		p.SendSwitchTableRsp(model.ErrExitTableFail)
// 		return
// 	}
//
// 	if !newTable.ThrowInto(p) {
// 		p.SendSwitchTableRsp(model.ErrEnterTableFail)
// 		return
// 	}
// 	// 推送换桌成功消息
// 	p.SendSwitchTableRsp(nil)
// 	return true
// }
//
// func (m *TableManager) ThrowInto(p *gplayer.Player) bool {
// 	if p == nil {
// 		return false
// 	}
// 	best := m.getTopTable(p, false)
// 	if best == nil {
// 		return false
// 	}
// 	return best.ThrowInto(p)
// }
//
// func (m *TableManager) getTopTable(p *gplayer.Player, isSwitchTable bool) *gtable.Table {
//
// 	var best *gtable.Table
// 	var oldTableID = p.GetTableID()
//
// 	for _, t := range m.tableList {
// 		if t == nil {
// 			continue
// 		}
// 		if t.IsFull() {
// 			continue
// 		}
// 		if !t.CanEnter(p) {
// 			continue
// 		}
// 		if isSwitchTable && t.ID == oldTableID {
// 			continue
// 		}
// 		if best != nil && t.GetSitCnt() <= best.GetSitCnt() {
// 			continue
// 		}
// 		best = t
// 	}
//
// 	if best == nil {
// 		log.Warn("无可用桌子，玩家ID: %d", p.GetPlayerID())
// 		return nil
// 	}
//
// 	return best
// }
//
// // CanEnterRoom 检查是否能进房
// func (m *TableManager) CanEnterRoom(p *gplayer.Player, in *v1.LoginReq) (err *errors.Error) {
// 	if p == nil {
// 		return model.ErrPlayerNotFound
// 	}
//
// 	// 校验token
// 	if in.Token == "" {
// 		return model.ErrTokenFail
// 	}
//
// 	// room limit
// 	c := m.repo.GetRoomConfig().Game
// 	if err = checkRoomLimit(p, c); err != nil {
// 		return err
// 	}
// 	return nil
// }
//
// func checkRoomLimit(p *gplayer.Player, c *conf.Room_Game) (err *errors.Error) {
// 	money := p.GetMoney()
// 	vip := p.GetVipGrade()
// 	if money < c.MinMoney {
// 		return model.ErrMoneyBelowMinLimit
// 	}
// 	if money > c.MaxMoney && c.MaxMoney != -1 {
// 		return model.ErrMoneyOverMaxLimit
// 	}
// 	if money < c.BaseMoney {
// 		return model.ErrMoneyBelowBaseLimit
// 	}
// 	if vip < c.VipLimit {
// 		return model.ErrVipLimit
// 	}
// 	return nil
// }
