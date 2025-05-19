package room

import (
	"context"

	"github.com/google/wire"

	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/gplayer"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/gtable"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/iface"
)

// 确保Room实现iface.IRoomRepo
var _ iface.IRoomRepo = (*Room)(nil)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(New)

var (
	defaultPendingNum = 10000
)

type Room struct {
	worker    work.IWorkStore
	playerMgr *gplayer.Manager
	tableMgr  *gtable.TableManager
	logger    log.Logger
}

func New(c *conf.Room, logger log.Logger) *Room {
	r := &Room{}
	r.playerMgr = gplayer.NewPlayerManager(c, r)
	r.tableMgr = gtable.NewTableManager(c, r)
	r.worker = work.NewWorkStore(context.Background(), defaultPendingNum)
	return r
}

func (r *Room) Start() {
	if err := r.worker.Start(); err != nil {
		panic(err)
	}
	r.playerMgr.Start()
	r.tableMgr.Start()
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
	log.Infof("Room handling table event:%+v %+v", tableID, evt)
}

func (r *Room) OnPlayerLeave(playerID string) {
	log.Infof("Room handling player leave:%+v", playerID)
}

func (r *Room) SubmitEvent(eventID iface.EventID, cb iface.EventCallback) {
	switch eventID {
	default:
	}

	// ...
	//if cb != nil {
	//	cb(eventID)
	//}
}
