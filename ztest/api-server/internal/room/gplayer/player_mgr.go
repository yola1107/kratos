package gplayer

import (
	"sync"
)

type Manager struct {
	playerMap sync.Map // key: playerID, value: *Player
}

func NewManager() *Manager {
	return &Manager{}
}

func (pm *Manager) Start() {
	// 启动相关定时、回收、广播逻辑
}

func (pm *Manager) Stop() {
	// 停止回收、清理状态
}
