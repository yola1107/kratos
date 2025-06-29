package robot

import (
	"sync"
	"time"

	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/table"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
)

const (
	defaultMaxBatchCnt   = 100
	defaultLoadInterval  = 5 * time.Second
	defaultLoginInterval = 5 * time.Second
)

type Manager struct {
	repo       Repo
	c          *conf.Room
	all        sync.Map // key: playerID, value: *Player
	free       sync.Map // key: playerID, value: *Player
	nextID     int64    //
	timerIDMap sync.Map // map[any]int64
}

func NewManager(c *conf.Room, repo Repo) *Manager {
	return &Manager{
		c:      c,
		repo:   repo,
		nextID: c.Robot.IdBegin,
	}
}

func (m *Manager) Start() error {
	timer := m.repo.GetTimer()
	loadID := timer.Forever(defaultLoadInterval, m.Load)
	loginID := timer.Forever(defaultLoginInterval, m.Login)
	m.timerIDMap.Store(loadID, loadID)
	m.timerIDMap.Store(loginID, loginID)
	return nil
}

func (m *Manager) Stop() {
	timer := m.repo.GetTimer()
	m.timerIDMap.Range(func(_, value any) bool {
		id, ok := value.(int64)
		if !ok {
			return true
		}
		if timer != nil {
			timer.Cancel(id)
		}
		return true
	})
}

func (m *Manager) Load() {
	cfg := m.c.Robot
	if !cfg.Open {
		return
	}

	remain := cfg.Num - int32(m.countAll())
	idEnd := cfg.IdBegin + int64(cfg.Num*2)
	if remain <= 0 || m.nextID > idEnd {
		return
	}

	for i := int32(0); i < min(defaultMaxBatchCnt, remain); i++ {
		id := m.nextID
		m.nextID++
		rob, err := m.repo.CreateRobot(&player.Raw{
			ID:      id,
			IsRobot: true,
		})
		if err != nil {
			log.Errorf("%v", err)
			continue
		}
		if rob == nil {
			log.Debugf("robot %d is nil", id)
			continue
		}
		m.Reset(rob)
		m.all.Store(id, rob)
		m.free.Store(id, rob)
	}
}

func (m *Manager) Login() {
	if !m.c.Robot.Open {
		return
	}

	tables := m.repo.GetTableList()
	if len(tables) == 0 {
		return
	}

	m.free.Range(func(_, val any) bool {
		p, ok := val.(*player.Player)
		if !ok {
			return true
		}
		if err := table.CheckRoomLimit(p, m.c.Game); err != nil {
			return true
		}
		for _, tb := range tables {
			if m.Enter(p, tb) {
				break
			}
		}
		return true
	})
}

func (m *Manager) Enter(p *player.Player, tb *table.Table) (enter bool) {
	if p.GetTableID() > 0 {
		return
	}
	if tb.IsFull() {
		return
	}
	if !tb.CanEnterRobot(p) {
		return
	}
	if !tb.ThrowInto(p) {
		return
	}
	m.free.Delete(p.GetPlayerID())
	return true
}

func (m *Manager) Leave(uid int64) bool {
	val, ok := m.all.Load(uid)
	if !ok {
		return false
	}
	p, ok := val.(*player.Player)
	if !ok {
		m.all.Delete(uid)
		m.free.Delete(uid)
		return false
	}
	if _, free := m.free.Load(uid); free {
		return true // 已经是空闲
	}
	m.Reset(p)
	m.free.Store(uid, p)
	return true
}

func (m *Manager) Reset(p *player.Player) {
	m.updateMoney(p)
}

func (m *Manager) updateMoney(p *player.Player) {
	if p.GetTableID() > 0 {
		return
	}
	money := p.GetBaseData().Money
	minMoney := max(m.c.Robot.MinMoney, m.c.Game.MinMoney)
	maxMoney := min(m.c.Robot.MaxMoney, m.c.Game.MaxMoney)
	if money < minMoney || money > maxMoney {
		money = ext.RandFloat(minMoney, maxMoney)
		p.GetBaseData().Money = float64(int64(money))
	}
}

func (m *Manager) Counter() (all, free, gaming int) {
	if !m.c.Robot.Open || m.c.Robot.Num <= 0 {
		return
	}

	all = m.countAll()
	free = m.countFree()
	return all, free, all - free
}

func (m *Manager) countAll() int {
	count := 0
	m.all.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

func (m *Manager) countFree() int {
	count := 0
	m.free.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}
