package room

import (
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gplayer"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gtable"
)

var (
	playerMgr *gplayer.Manager
	tableMgr  *gtable.Manager
)

func Start() {
	log.Infof("start server:%s version:%s GameID:%d ArenaID:%d ServerID:%s",
		conf.Name, conf.Version, conf.GameID, conf.ArenaID, conf.ServerID)

	playerMgr = gplayer.NewManager()
	tableMgr = gtable.NewManager()

	playerMgr.Start()
	tableMgr.Start()
}

func Stop() {
	playerMgr.Stop()
	tableMgr.Stop()
	log.Info("room stopped.")
}

func GetTable(tableID int32) *gtable.Table {
	return tableMgr.GetTable(tableID)
}

func ThrowInto(p *gplayer.Player) bool {
	return tableMgr.ThrowInto(p)
}

func SwitchTable(p *gplayer.Player) bool {
	return tableMgr.SwitchTable(p)
}
