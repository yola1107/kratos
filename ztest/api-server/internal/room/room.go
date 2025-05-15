package room

import (
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/playermgr"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/tablemgr"
)

func Init(Name, Version string) {
	log.Infof("start server:%s version:%+v GameID:%d ArenaID:%d ServerID:%s",
		Name, Version, conf.GameID, conf.ArenaID, conf.ServerID)

	playermgr.Init()
	tablemgr.Init()
}
