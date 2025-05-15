package playermgr

import (
	"sync"

	"github.com/yola1107/kratos/v2/log"
)

type PlayerMgr struct {
	playerMap sync.Map // key: playerID, value: *Player
}

func New() *PlayerMgr {
	log.Infof("PlayerMgr init. ")
	return &PlayerMgr{}
}

func (pm *PlayerMgr) Start() {
	// 启动相关定时、回收、广播逻辑
}

func (pm *PlayerMgr) Stop() {
	// 停止回收、清理状态
}
