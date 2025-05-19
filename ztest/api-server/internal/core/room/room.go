package room

import (
	"context"
	"fmt"

	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/gplayer"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/gtable"
)

//import (
//	"github.com/yola1107/kratos/v2/log"
//	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
//	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gplayer"
//	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gtimer"
//	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/tablemgr"
//)
//
//func Init() {
//	log.Infof("start server:%s version:%s GameID:%d ArenaID:%d ServerID:%s",
//		conf.Name, conf.Version, conf.GameID, conf.ArenaID, conf.ServerID)
//
//	gtimer.Init()
//	gplayer.Init()
//	tablemgr.Init()
//}
//
//func Close() {
//	gtimer.Close()
//}

type Room struct {
	tableMgr  *gtable.TableManager
	playerMgr *gplayer.Manager
	worker    work.IWorkStore
}

//func New(tableMgr *gtable.TableManager, playerMgr *gplayer.Manager) *Room {

func New() *Room {
	log.Infof("start server:%s version:%s ", conf.Name, conf.Version)
	c := conf.GetRC()
	r := &Room{}
	r.playerMgr = gplayer.NewPlayerManager(c, r)
	r.tableMgr = gtable.NewTableManager(c, r)
	r.worker = work.NewWorkStore(context.Background(), 10000)

	return r
}

func (r *Room) Start() {
	if err := r.worker.Start(); err != nil {
		panic(err)
	}
	r.playerMgr.Start()
	r.tableMgr.Start()
	log.Infof("Room start. Name=\"%s\" GameID=%d ArenaID=%d ServerID=%s",
		conf.Name, conf.GameID, conf.ArenaID, conf.ServerID)
}

func (r *Room) Close() {
	r.playerMgr.Close()
	r.tableMgr.Close()
	r.worker.Stop()
	log.Infof("Room stop.")
}

func (r *Room) GetLoop() work.ITaskLoop {
	return r.worker
}
func (r *Room) GetTimer() work.ITaskScheduler {
	return r.worker
}
func (r *Room) OnTableEvent(tableID string, evt string) {
	fmt.Println("Room handling table event:", tableID, evt)
}
func (r *Room) OnPlayerLeave(playerID string) {
	fmt.Println("Room handling player leave:", playerID)
}
