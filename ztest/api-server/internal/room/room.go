package room

import (
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gplayer"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gtable"
)

var (
	ins *Room
)

type Room struct {
	playerMgr *gplayer.Manager
	tableMgr  *gtable.Manager
}

func Init() *Room {
	log.Infof("start server:%s version:%s GameID:%d ArenaID:%d ServerID:%s",
		conf.Name, conf.Version, conf.GameID, conf.ArenaID, conf.ServerID)

	ins = &Room{
		playerMgr: gplayer.NewManager(),
		tableMgr:  gtable.NewManager(),
	}
	ins.Start()
	return ins
}

func GetInstance() *Room {
	return ins
}

func (r *Room) Start() {
	r.playerMgr.Start()
	r.tableMgr.Start()
}

func (r *Room) Close() {
	r.playerMgr.Close()
	r.tableMgr.Close()
	log.Info("room stopped.")
}

func (r *Room) GetTable(id int32) *gtable.Table {
	return r.tableMgr.GetTable(id)
}

func (r *Room) ThrowInto(p *gplayer.Player) bool {
	return r.tableMgr.ThrowInto(p)
}

func (r *Room) SwitchTable(p *gplayer.Player) bool {
	return r.tableMgr.SwitchTable(p)
}
