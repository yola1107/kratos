package room

import (
	"sync"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/player"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/playermgr"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/tablemgr"
)

var (
	ins  *Room
	once sync.Once
)

type Room struct {
	PlayerMgr *playermgr.PlayerMgr
	TableMgr  *tablemgr.TableMgr
}

func GetInstance() *Room {
	once.Do(func() {
		ins = &Room{
			PlayerMgr: playermgr.New(),
			TableMgr:  tablemgr.New(),
		}
	})
	return ins
}

func (r *Room) Start() {
	r.PlayerMgr.Start()
	r.TableMgr.Start()
	log.Infof("room started.")
}

func (r *Room) Stop() {
	r.PlayerMgr.Stop()
	r.TableMgr.Stop()
	log.Info("room stopped.")
}

func ThrowInto(p *player.Player) bool {
	r := GetInstance()
	return r.TableMgr.ThrowInto(p)
}

func SwitchTable(p *player.Player) bool {
	r := GetInstance()
	return r.TableMgr.SwitchTable(p)
}
