package gplayer

import (
	"sync"

	"github.com/google/wire"

	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/iface"
)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(NewPlayerManager)

type Manager struct {
	playerMap sync.Map // key: playerID, value: *Player
	repo      iface.IRoomRepo
}

func NewPlayerManager(c *conf.Room, repo iface.IRoomRepo) *Manager {
	//log.Infof("playerMgr init. ")
	return &Manager{
		playerMap: sync.Map{},
		repo:      repo,
	}
}

func (m *Manager) Start() error {
	// 启动相关定时、回收、广播逻辑
	//log.Infof("PlayerMgr start")
	return nil
}

func (m *Manager) Close() {
	// 停止回收、清理状态
}
