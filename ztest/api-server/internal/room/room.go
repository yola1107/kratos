package room

import (
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/player"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/table"
)

var (
	playerMgr *player.Manager
	tableMgr  *table.Manager
)

// Start 启动房间模块
func Start() {
	playerMgr = player.NewManager()
	tableMgr = table.NewManager()

	playerMgr.Start()
	tableMgr.Start()
	log.Infof("room started.")
}

// Stop 停止房间模块
func Stop() {
	playerMgr.Stop()
	tableMgr.Stop()
	log.Info("room stopped.")
}

// ThrowInto 将玩家分配进桌子
func ThrowInto(p *player.Player) bool {
	return tableMgr.ThrowInto(p)
}

// SwitchTable 切桌
func SwitchTable(p *player.Player) bool {
	return tableMgr.SwitchTable(p)
}
