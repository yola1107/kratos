package gplayer

import (
	"context"
	"sync"

	"github.com/yola1107/kratos/v2/errors"
	"github.com/yola1107/kratos/v2/transport/websocket"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/iface"
)

type PlayerManager struct {
	playerMap sync.Map // key: playerID, value: *Player
	repo      iface.IRoomRepo
}

type PlayerRaw struct {
	ID      int64
	IP      string
	Session *websocket.Session
}

func NewPlayerManager(c *conf.Room, repo iface.IRoomRepo) *PlayerManager {
	return &PlayerManager{
		playerMap: sync.Map{},
		repo:      repo,
	}
}

func (m *PlayerManager) Start() error {
	// 启动相关定时、回收、广播逻辑
	m.repo.OnPlayerLeave("abc") // test
	_, _ = m.repo.GetDataRepo().FindByID(context.Background(), 1001)
	return nil
}

func (m *PlayerManager) Close() {
	// 停止回收、清理状态
}

// ExistPlayer exit
func (m *PlayerManager) ExistPlayer(id int64) bool {
	_, ok := m.playerMap.Load(id)
	return ok
}

// GetPlayerByID 获取玩家
func (m *PlayerManager) GetPlayerByID(id int64) *Player {
	if p, ok := m.playerMap.Load(id); ok {
		return p.(*Player)
	}
	return nil
}

func (m *PlayerManager) GetPlayerBySessionID(id string) *Player {
	var result *Player
	m.playerMap.Range(func(_, value interface{}) bool {
		p := value.(*Player)
		if p.GetSessionID() == id {
			result = p
			return false // 终止遍历
		}
		return true
	})
	return result
}

func (m *PlayerManager) Range(cb func(id int64, p *Player)) {
	if cb == nil {
		return
	}
	m.playerMap.Range(func(id, value interface{}) bool {
		if p := value.(*Player); p != nil {
			cb(p.GetPlayerID(), p)
		}
		return true
	})
}

/*

	玩家行为回调
		OnLoginReq登录 CreatePlayer
		OnLogoutReq登出/踢人/关服 LogoutGame
*/

// CreatePlayer 创建玩家
func (m *PlayerManager) CreatePlayer(raw *PlayerRaw) (*Player, *errors.Error) {
	return nil, nil
}

// LogoutGame 释放玩家
func (m *PlayerManager) LogoutGame(p *Player, code int32, msg string) {
	if p == nil {
		return
	}
	m.playerMap.Delete(p.GetPlayerID())
	m.repo.OnPlayerLeave(p.GetSessionID()) // test

	go func() {
		// p.UnSerialize(code, msg)
	}()
}
