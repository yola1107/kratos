package player

import (
	"sync"

	"github.com/yola1107/kratos/v2/log"
)

type Manager struct {
	playerMap sync.Map // key: playerID, value: *Player
}

func NewManager() *Manager {
	log.Infof("PlayerMgr init. ")
	return &Manager{}
}

func (pm *Manager) Start() {
	// 启动相关定时、回收、广播逻辑
}

func (pm *Manager) Stop() {
	// 停止回收、清理状态
}
