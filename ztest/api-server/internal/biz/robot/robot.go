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
	defaultMaxBatchCnt   = 10
	defaultInterval      = time.Second * 5
	defaultLoginInterval = time.Second * 2
)

type Manager struct {
	repo Repo
	c    *conf.Room

	all  sync.Map // key: playerID, value: *Player
	free sync.Map // key: playerID, value: *Player

	nextID  int64
	timerID int64 // 定时任务ID，用于取消
}

func NewManager(c *conf.Room, repo Repo) *Manager {
	return &Manager{
		c:      c,
		repo:   repo,
		nextID: c.Robot.IdBegin,
	}
}

func (m *Manager) Start() error {
	m.timerID = m.repo.GetTimer().ForeverNow(defaultInterval, m.Load)
	_ = m.repo.GetTimer().Forever(defaultLoginInterval, m.Login)
	_ = m.repo.GetTimer().Forever(time.Second*30, func() {
		if m.c.Robot.Open && m.c.Robot.Num > 0 {
			all, free := m.countAll(), m.countFree()
			log.Info("<AI> all:%v free:%v gaming:%v", all, free, all-free)
		}
	})
	return nil
}

func (m *Manager) Stop() {}

// countAll returns how many robots are currently stored.
func (m *Manager) countAll() int {
	count := 0
	m.all.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

// countFree returns how many robots are currently free.
func (m *Manager) countFree() int {
	count := 0
	m.free.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

func (m *Manager) Load() {
	cfg := m.c.Robot
	if !cfg.Open {
		return
	}

	// 达到目标数量，取消定时任务
	remain := cfg.Num - int32(m.countAll())
	IdEnd := cfg.IdBegin + int64(cfg.Num*2)
	if remain <= 0 || m.nextID > IdEnd {
		m.repo.GetTimer().Cancel(m.timerID)
		log.Infof("load robots success. Num=%d", cfg.Num)
		return
	}

	// 批量加载AI基础数据
	batchSize := min(defaultMaxBatchCnt, remain)
	for i := int32(0); i < batchSize; i++ {
		id := m.nextID
		rob, err := m.repo.CreateRobot(&player.Raw{ID: id, IsRobot: true})
		m.nextID++

		if rob != nil && err == nil {
			m.all.Store(id, rob)
			m.free.Store(id, rob)
		}
	}
}

func (m *Manager) Login() {
	if !m.c.Robot.Open {
		return
	}

	tbList := m.repo.GetTableList()
	if len(tbList) == 0 {
		return
	}

	m.free.Range(func(id, val any) bool {
		p, ok := val.(*player.Player)
		if !ok {
			return true
		}
		if err := table.CheckRoomLimit(p, m.c.Game); err != nil {
			return true
		}
		for _, tb := range tbList {
			if ok = m.Enter(p, tb); ok {
				break
			}
		}
		return true
	})

}

func (m *Manager) Enter(p *player.Player, tb *table.Table) (can bool) {
	if p.GetTableID() > 0 {
		return // 已经进入了
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
	// delete free player
	m.free.Delete(p.GetPlayerID())
	return true
}

func (m *Manager) Leave(uid int64) bool {
	if _, ok := m.all.Load(uid); !ok {
		return false
	}
	if _, inFree := m.free.Load(uid); !inFree {
		v, ok := m.all.Load(uid)
		p, ok := v.(*player.Player)
		if !ok {
			// 脏数据
			m.all.Delete(uid)
			m.free.Delete(uid)
			return false
		}
		// store free player
		m.UpdateMoney(p)
		m.free.Store(uid, p)
	}
	return true
}

func (m *Manager) UpdateMoney(p *player.Player) {
	if p.GetTableID() > 0 {
		return
	}
	money := p.GetBaseData().Money
	if money > m.c.Robot.MaxMoney || money < m.c.Robot.MinMoney {
		money = ext.RandFloat(m.c.Robot.MinMoney, m.c.Robot.MaxMoney)
		p.GetBaseData().Money = money
	}
}

// func (r *Robot) Reset()       {}
