package gplayer

import (
	"sync"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/iface"
)

type Manager struct {
	playerMap sync.Map // key: playerID, value: *Player
	roomRepo  iface.IRoomRepo
}

func NewPlayerManager(c *conf.Room, repo iface.IRoomRepo) *Manager {
	log.Infof("playerMgr init. ")
	return &Manager{
		playerMap: sync.Map{},
		roomRepo:  repo,
	}
}

func (m *Manager) Start() {
	// 启动相关定时、回收、广播逻辑
	log.Infof("PlayerMgr start")

}

func (m *Manager) Close() {
	// 停止回收、清理状态
}
