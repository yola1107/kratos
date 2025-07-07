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
	defaultBatchLoadCount    = 100
	defaultBatchReleaseCount = 100
)

type Monitor struct {
	Max   int32
	Num   int32
	Gamed int32
	Free  int32
}
type Manager struct {
	conf *conf.Room
	repo Repo

	all  sync.Map // map[playerID]*player.Player
	free sync.Map // map[playerID]*player.Player
}

// NewManager 创建机器人管理器
func NewManager(c *conf.Room, repo Repo) *Manager {
	m := &Manager{
		conf: c,
		repo: repo,
	}
	return m
}

// Start 启动机器人管理器
func (m *Manager) Start() error {
	timer := m.repo.GetTimer()
	timer.Forever(5*time.Second, m.load)
	timer.Forever(3*time.Second, m.login)
	timer.Forever(10*time.Second, m.release)
	return nil
}

func (m *Manager) Stop() {}

// load 批量加载机器人，保持机器人数量符合配置
func (m *Manager) load() {
	cfg := m.conf.Robot
	if !cfg.Open || cfg.Num <= 0 {
		return
	}
	current := m.countAll()
	toLoad := min(cfg.Num-current, defaultBatchLoadCount)
	if toLoad <= 0 {
		return
	}

	idStart, idEnd := cfg.IdBegin, cfg.IdBegin+int64(cfg.Num*2)
	for id := idStart; id <= idEnd && toLoad > 0; id++ {
		if _, exists := m.all.Load(id); exists {
			continue
		}
		p, err := m.repo.CreateRobot(&player.Raw{ID: id, IsRobot: true})
		if err != nil || p == nil {
			log.Errorf("init robot error id=%d: %v", id, err)
			continue
		}
		m.reset(p)
		m.all.Store(id, p)
		m.free.Store(id, p)
		toLoad--
	}
}

// 释放多余机器人（空闲时释放）
func (m *Manager) release() {
	maxNum := int32(0)
	if cfg := m.conf.Robot; cfg.Open {
		maxNum = min(cfg.Num, cfg.MinPlayCount)
	}
	excess := m.countAll() - maxNum
	toRelease := min(excess, defaultBatchReleaseCount)
	if toRelease <= 0 {
		return
	}

	m.free.Range(func(k, v any) bool {
		p := v.(*player.Player)
		if p.GetTableID() > 0 {
			return true
		}
		m.all.Delete(k)
		m.free.Delete(k)
		toRelease--
		return toRelease > 0
	})
}

// login 尝试进入桌子
func (m *Manager) login() {
	if !m.conf.Robot.Open {
		return
	}

	tables := m.repo.GetTableList()
	if len(tables) == 0 {
		return
	}

	var index int
	m.free.Range(func(_, val any) bool {
		p, ok := val.(*player.Player)
		if !ok || p.GetTableID() > 0 {
			return true // 无效或已在桌上
		}
		if code, _ := table.CheckRoomLimit(p, m.conf.Game); code != 0 {
			return true
		}

		// 遍历桌子寻找能加入的
		for index < len(tables) {
			tb := tables[index]
			index++

			if m.Enter(p, tb) {
				m.free.Delete(p.GetPlayerID())
				return true // 下一个 AI
			}
		}

		return false // 桌子遍历完了，退出 Range
	})
}

func (m *Manager) Enter(p *player.Player, tb *table.Table) (enter bool) {
	if p == nil || tb == nil {
		return false
	}
	if p.GetTableID() > 0 {
		return true
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

	return true
}

// Leave 机器人离开桌子，放回空闲池
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
	// 已经在空闲池
	if _, alreadyFree := m.free.Load(uid); alreadyFree {
		return true
	}
	m.reset(p)
	m.free.Store(uid, p)
	return true
}

// Reset 重置机器人状态（比如金额）
func (m *Manager) reset(p *player.Player) {
	minMoney := max(m.conf.Robot.MinMoney, m.conf.Game.MinMoney)
	maxMoney := min(m.conf.Robot.MaxMoney, m.conf.Game.MaxMoney)
	money := p.GetBaseData().Money
	if money < minMoney || money > maxMoney {
		p.GetBaseData().Money = float64(int64(ext.RandFloat(minMoney, maxMoney)))
	}
}

// Monitor 返回当前机器人总数、空闲数和游戏中数量
func (m *Manager) Monitor() Monitor {
	if !m.conf.Robot.Open || m.conf.Robot.Num <= 0 {
		return Monitor{}
	}
	all := m.countAll()
	free := m.countFree()
	return Monitor{
		Max:   m.conf.Robot.Num,
		Num:   all,
		Free:  free,
		Gamed: all - free,
	}
}

func (m *Manager) countAll() int32 {
	var count int32
	m.all.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

func (m *Manager) countFree() int32 {
	var count int32
	m.free.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}
