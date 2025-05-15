package tablemgr

import (
	"time"

	"github.com/yola1107/kratos/v2/library/gtimer"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/playermgr/gplayer"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/tablemgr/gtable"
)

/*
桌子管理模块：
- 所有房间内行为均通过队列处理（入桌、换桌、桌内请求、定时逻辑）
*/

var (
	isRoomClosed bool                    // 房间是否已关闭
	tableList    []*gtable.Table         // 桌子列表（顺序用于遍历）
	tableMap     map[int32]*gtable.Table // tableID -> Table
)

func Init() {
	c := conf.GetTC()
	tableMap = make(map[int32]*gtable.Table)
	tableList = make([]*gtable.Table, c.TableNum)
	isRoomClosed = false

	for i := int32(1); i <= c.TableNum; i++ {
		tb := &gtable.Table{
			ID:     i,
			MaxCnt: int16(c.ChairNum),
		}
		tb.Init()
		tableMap[tb.ID] = tb
		tableList[i-1] = tb
	}
	gtimer.Forever(nil, time.Second/2, OnTimer)
	log.Infof("桌子初始化完成. 桌子数:%d 椅子:%d", c.TableNum, c.ChairNum)
}

func OnTimer() {
	for _, t := range tableList {
		if t != nil && t.IsRunning() {
			t.OnTimer()
		}
	}
}

func GetTable(tableID int32) *gtable.Table {
	return tableMap[tableID]
}

func ThrowInto(p *gplayer.Player) bool {
	best := getTopTable(p, false)
	if best == nil {
		log.Warn("ThrowInto失败, 玩家ID: %d", p.GetID())
		return false
	}
	return best.ThrowInto(p, false)
}

func SwitchTable(p *gplayer.Player) bool {
	old := GetTable(p.GetTableID())
	if old != nil {
		//old.Leave(p.GetChairID())
	}

	best := getTopTable(p, true)
	if best == nil {
		log.Warn("SwitchTable失败, 玩家ID: %d", p.GetID())
		return false
	}
	return best.ThrowInto(p, true)
}

func getTopTable(p *gplayer.Player, canSwitch bool) *gtable.Table {
	var best *gtable.Table
	var old *gtable.Table

	if canSwitch {
		old = GetTable(p.GetTableID())
	}

	for _, v := range tableList {
		if v == nil || v == old || v.IsFull() {
			continue
		}
		if best != nil && v.GetSitCnt() <= best.GetSitCnt() {
			continue
		}
		best = v
	}
	if best == nil {
		log.Warn("无可用桌子, 玩家ID: %d", p.GetID())
	}
	return best
}
