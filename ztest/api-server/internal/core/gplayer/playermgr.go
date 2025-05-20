package gplayer

import (
	"sync"

	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/iface"
)

type Manager struct {
	playerMap sync.Map // key: playerID, value: *Player
	repo      iface.IRoomRepo
}

func NewManager(c *conf.Room, repo iface.IRoomRepo) *Manager {
	//log.Infof("playerMgr init. ")
	return &Manager{
		playerMap: sync.Map{},
		repo:      repo,
	}
}

func (m *Manager) Start() error {
	// 启动相关定时、回收、广播逻辑
	//log.Infof("PlayerMgr start")
	m.repo.OnPlayerLeave("abc")
	return nil
}

func (m *Manager) Close() {
	// 停止回收、清理状态
}
