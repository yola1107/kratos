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
	defaultBatchLoadCnt    = 100
	defaultBatchLoginCnt   = 100
	defaultBatchReleaseCnt = 100
	defaultLoadInterval    = 5 * time.Second
	defaultLoginInterval   = 3 * time.Second
	failRetryInterval      = 30 * time.Second
)

type robotMeta struct {
	lastUsed        time.Time
	lastFailEnterAt sync.Map // map[tableID]int64 (Unix)
}

func (rm *robotMeta) ShouldRetry(tableID int32) bool {
	v, _ := rm.lastFailEnterAt.LoadOrStore(tableID, int32(0))
	last := v.(int64)
	return time.Now().Unix()-last >= int64(failRetryInterval.Seconds())
}

func (rm *robotMeta) MarkFail(tableID int32) {
	rm.lastFailEnterAt.Store(tableID, time.Now().Unix())
}

type Manager struct {
	repo       Repo
	c          *conf.Room
	all        sync.Map // playerID → *player.Player
	free       sync.Map // playerID → *player.Player
	meta       sync.Map // playerID → *robotMeta
	timerIDMap sync.Map // timerID → timerID
}

func NewManager(c *conf.Room, repo Repo) *Manager {
	return &Manager{
		c:    c,
		repo: repo,
	}
}

func (m *Manager) Start() error {
	timer := m.repo.GetTimer()
	loadID := timer.Forever(defaultLoadInterval, m.load)
	loginID := timer.Forever(defaultLoginInterval, m.login)
	releaseID := timer.Forever(60*time.Second, m.release)

	m.timerIDMap.Store(loadID, loadID)
	m.timerIDMap.Store(loginID, loginID)
	m.timerIDMap.Store(releaseID, releaseID)
	return nil
}

func (m *Manager) Stop() {
	timer := m.repo.GetTimer()
	m.timerIDMap.Range(func(_, value any) bool {
		if id, ok := value.(int64); ok && timer != nil {
			timer.Cancel(id)
		}
		return true
	})
}

func (m *Manager) load() {
	cfg := m.c.Robot
	if !cfg.Open || cfg.Num <= 0 {
		return
	}

	// 计算加载的数量
	count := min(cfg.Num-m.countAll(), defaultBatchLoadCnt)
	if count <= 0 {
		return
	}

	idBegin, idEnd := cfg.IdBegin, cfg.IdBegin+int64(cfg.Num*2) // [begin,begin*2]
	for id := idBegin; id <= idEnd; id++ {
		if _, ok := m.all.Load(id); ok {
			continue
		}
		rob, err := m.repo.CreateRobot(&player.Raw{
			ID:      id,
			IsRobot: true,
		})
		if err != nil || rob == nil {
			log.Errorf("create robot error: %v", err)
			continue
		}
		m.Reset(rob)
		m.all.Store(id, rob)
		m.free.Store(id, rob)
		m.meta.Store(id, &robotMeta{lastUsed: time.Now()})
		if count--; count <= 0 {
			return
		}
	}
}

func (m *Manager) release() {
	limitNum := int32(0)
	if cfg := m.c.Robot; cfg.Open {
		limitNum = cfg.Num
	}
	// 计算释放数量
	count := min(m.countAll()-limitNum, defaultBatchReleaseCnt)
	if count <= 0 {
		return
	}
	m.free.Range(func(k, v any) bool {
		p, ok := v.(*player.Player)
		if !ok || p.GetTableID() > 0 {
			return true
		}
		uid := k.(int64)
		m.all.Delete(uid)
		m.free.Delete(uid)
		m.meta.Delete(uid)
		count--
		return count > 0
	})
}

// func (m *Manager) login() {
// 	if !m.c.Robot.Open {
// 		return
// 	}
//
// 	tables := m.repo.GetTableList()
// 	if len(tables) == 0 {
// 		return
// 	}
//
// 	count := 0
//
// 	m.free.Range(func(_, val any) bool {
// 		if count >= defaultBatchLoginCnt {
// 			return false
// 		}
// 		p, ok := val.(*player.Player)
// 		if !ok || p.GetTableID() > 0 {
// 			return true
// 		}
// 		if err := table.CheckRoomLimit(p, m.c.Game); err != nil {
// 			return true
// 		}
// 		for _, tb := range tables {
// 			if m.Enter(p, tb) {
// 				count++
// 				m.free.Delete(p.GetPlayerID())
// 				break
// 			}
// 		}
// 		return true
// 	})
// }

func (m *Manager) login() {
	if !m.c.Robot.Open {
		return
	}

	tables := m.repo.GetTableList()
	if len(tables) == 0 {
		return
	}

	count := 0
	m.free.Range(func(_, val any) bool {
		if count >= defaultBatchLoginCnt {
			return false
		}
		p, ok := val.(*player.Player)
		if !ok || p.GetTableID() > 0 {
			return true
		}
		if err := table.CheckRoomLimit(p, m.c.Game); err != nil {
			return true
		}

		metaVal, _ := m.meta.LoadOrStore(p.GetPlayerID(), &robotMeta{})
		meta := metaVal.(*robotMeta)

		for _, tb := range tables {
			if !meta.ShouldRetry(tb.ID) {
				continue
			}
			if tb.IsFull() || !tb.CanEnterRobot(p) {
				meta.MarkFail(tb.ID)
				continue
			}
			if tb.ThrowInto(p) {
				m.free.Delete(p.GetPlayerID())
				meta.lastUsed = time.Now()
				count++
				break
			}
			meta.MarkFail(tb.ID)
		}
		return true
	})
}

func (m *Manager) Enter(p *player.Player, tb *table.Table) (entered bool) {
	if p.GetTableID() > 0 || tb.IsFull() || !tb.CanEnterRobot(p) {
		return false
	}
	return tb.ThrowInto(p)
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
		m.meta.Delete(uid)
		return false
	}
	if _, alreadyFree := m.free.Load(uid); alreadyFree {
		return true
	}
	// 重置信息
	m.Reset(p)
	m.free.Store(uid, p)
	// meta, _ := m.meta.LoadOrStore(uid, &robotMeta{})
	// meta.(*robotMeta).lastUsed = time.Now()
	return true
}

func (m *Manager) Reset(p *player.Player) {
	m.updateMoney(p)
}

func (m *Manager) updateMoney(p *player.Player) {
	if p.GetTableID() > 0 {
		return
	}
	minMoney := max(m.c.Robot.MinMoney, m.c.Game.MinMoney)
	maxMoney := min(m.c.Robot.MaxMoney, m.c.Game.MaxMoney)
	if money := p.GetBaseData().Money; money < minMoney || money > maxMoney {
		p.GetBaseData().Money = float64(int64(ext.RandFloat(minMoney, maxMoney)))
	}
}

func (m *Manager) Counter() (all, free, gaming int32) {
	if !m.c.Robot.Open || m.c.Robot.Num <= 0 {
		return
	}
	all = m.countAll()
	free = m.countFree()
	return all, free, all - free
}

func (m *Manager) countAll() int32 {
	count := int32(0)
	m.all.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

func (m *Manager) countFree() int32 {
	count := int32(0)
	m.free.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}
