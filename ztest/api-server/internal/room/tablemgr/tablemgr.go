package tablemgr

import (
	"time"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gtimer"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/playermgr/gplayer"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/tablemgr/gtable"
)

var (
	tableList []*gtable.Table
	tableMap  map[int32]*gtable.Table
	closed    bool
)

func Init() {
	c := conf.GetTC()
	tableList = make([]*gtable.Table, c.TableNum)
	tableMap = make(map[int32]*gtable.Table)
	for i := int32(1); i <= c.TableNum; i++ {
		tb := &gtable.Table{ID: i, MaxCnt: int16(c.ChairNum)}
		tb.Init()
		tableMap[i] = tb
		tableList[i-1] = tb
	}
	gtimer.GetWorkStore().Forever(time.Second/2, onTimer)
	log.Infof("tablemgr init. tables=%d chairs=%d", c.TableNum, c.ChairNum)
	return
}

func onTimer() {
	//log.Infof("onTimer")
	if closed {
		return
	}
	for _, t := range tableList {
		if t.IsRunning() {
			t.OnTimer()
		}
	}
}

func GetTable(id int32) *gtable.Table {
	return tableMap[id]
}

func ThrowInto(p *gplayer.Player) bool {
	best := getTopTable(p, false)
	if best == nil {
		return false
	}
	return best.ThrowInto(p)
}

func getTopTable(p *gplayer.Player, canSwitch bool) *gtable.Table {
	var best, old *gtable.Table
	if canSwitch {
		old = GetTable(p.GetTableID())
	}
	for _, t := range tableList {
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

func SwitchTable(p *gplayer.Player) bool {
	return false
}
