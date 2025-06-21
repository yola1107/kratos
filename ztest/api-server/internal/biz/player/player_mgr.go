package player

import (
	"sync"

	"github.com/yola1107/kratos/v2/log"
)

type Manager struct {
	players sync.Map // key: playerID, value: *Player
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Add(p *Player) {
	m.players.Store(p.GetPlayerID(), p)
}

func (m *Manager) Has(id int64) bool {
	_, ok := m.players.Load(id)
	return ok
}

func (m *Manager) GetByID(id int64) *Player {
	if p, ok := m.players.Load(id); ok {
		return p.(*Player)
	}
	return nil
}

func (m *Manager) GetBySessionID(id string) *Player {
	var result *Player
	m.players.Range(func(_, value interface{}) bool {
		p := value.(*Player)
		if p.GetSessionID() == id {
			result = p
			return false
		}
		return true
	})
	return result
}

func (m *Manager) Remove(id int64) {
	// if p, ok := m.players.Load(id); ok {
	// 	p.(*Player).Close() // 假设实现
	// }
	m.players.Delete(id)
}

func (m *Manager) All() []*Player {
	var result []*Player
	m.players.Range(func(_, value interface{}) bool {
		result = append(result, value.(*Player))
		return true
	})
	return result
}

func (m *Manager) Count() int {
	count := 0
	m.players.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

func (m *Manager) Counter() {
	all := 0
	offline := 0

	m.players.Range(func(_, value interface{}) bool {
		all++
		p := value.(*Player)
		if p != nil && p.gameData != nil && p.gameData.isOffline {
			offline++
		}

		return true
	})

	log.Infof("<Player> Total:%d Offline:%d", all, offline)
}
