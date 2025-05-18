package room

import (
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gtimer"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/playermgr"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/tablemgr"
)

func Init() {
	log.Infof("start server:%s version:%s GameID:%d ArenaID:%d ServerID:%s",
		conf.Name, conf.Version, conf.GameID, conf.ArenaID, conf.ServerID)

	gtimer.Init()
	playermgr.Init()
	tablemgr.Init()
}

func Close() {
	gtimer.Close()
}
