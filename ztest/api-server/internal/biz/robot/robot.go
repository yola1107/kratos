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
	defaultBatchLoginCount   = 100
	defaultBatchReleaseCount = 100
	defaultLoadInterval      = 5 * time.Second
	defaultLoginInterval     = 3 * time.Second
	failRetryInterval        = 3 * time.Second
)

type MetaData struct {
	lastUsed      time.Time
	lastFailEnter sync.Map // map[tableID]int64, value is unix timestamp of last failure
}

// ShouldRetry 判断当前机器人是否可以重试进入指定的桌子
func (md *MetaData) ShouldRetry(tableID int32) bool {
	val, _ := md.lastFailEnter.LoadOrStore(tableID, int64(0))
	lastFailUnix := val.(int64)
	return time.Now().Unix()-lastFailUnix >= int64(failRetryInterval.Seconds())
}

// MarkFail 标记机器人进入桌子失败的时间戳
func (md *MetaData) MarkFail(tableID int32) {
	md.lastFailEnter.Store(tableID, time.Now().Unix())
}

// TryEnterTable 让机器人尝试进入列表中的某个桌子，返回成功进入的桌子实例
func (md *MetaData) TryEnterTable(p *player.Player, tables []*table.Table) *table.Table {
	for _, tb := range tables {
		// if !md.ShouldRetry(tb.ID) {
		// 	continue
		// }
		if tb.IsFull() || !tb.CanEnterRobot(p) {
			md.MarkFail(tb.ID)
			continue
		}
		if tb.ThrowInto(p) {
			md.lastUsed = time.Now()
			return tb
		}
		md.MarkFail(tb.ID)
	}
	return nil
}

type Manager struct {
	repo       Repo
	conf       *conf.Room
	all        sync.Map // map[playerID]*player.Player 所有机器人
	free       sync.Map // map[playerID]*player.Player 空闲机器人（未进入桌子）
	meta       sync.Map // map[playerID]*MetaData 机器人元数据
	timerIDMap sync.Map // map[timerID]timerID 定时器ID管理
}

// NewManager 创建机器人管理器实例
func NewManager(c *conf.Room, repo Repo) *Manager {
	return &Manager{
		conf: c,
		repo: repo,
	}
}

// Start 启动机器人管理器，开启定时任务加载、登录、释放机器人
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

// Stop 停止所有定时器
func (m *Manager) Stop() {
	timer := m.repo.GetTimer()
	m.timerIDMap.Range(func(_, val any) bool {
		if id, ok := val.(int64); ok {
			timer.Cancel(id)
		}
		return true
	})
}

// load 批量加载机器人，保持机器人数量符合配置
func (m *Manager) load() {
	cfg := m.conf.Robot
	if !cfg.Open || cfg.Num <= 0 {
		return
	}

	currentCount := m.countAll()
	countToLoad := min(cfg.Num-currentCount, defaultBatchLoadCount)
	if countToLoad <= 0 {
		return
	}

	idStart, idEnd := cfg.IdBegin, cfg.IdBegin+int64(cfg.Num*2)
	for id := idStart; id <= idEnd && countToLoad > 0; id++ {
		if _, exists := m.all.Load(id); exists {
			continue
		}
		if _, err := m.initRobot(id); err != nil {
			log.Errorf("robot init error, id=%d: %v", id, err)
			continue
		}
		countToLoad--
	}
}

// initRobot 初始化单个机器人，存储到管理器中
func (m *Manager) initRobot(id int64) (*player.Player, error) {
	rob, err := m.repo.CreateRobot(&player.Raw{ID: id, IsRobot: true})
	if err != nil || rob == nil {
		return nil, err
	}
	m.Reset(rob)
	m.all.Store(id, rob)
	m.free.Store(id, rob)
	m.meta.Store(id, &MetaData{lastUsed: time.Now()})
	return rob, nil
}

// release 批量释放空闲机器人，保持机器人数量符合配置限制
func (m *Manager) release() {
	limitNum := int32(0)
	if cfg := m.conf.Robot; cfg.Open {
		limitNum = cfg.Num
	}
	excess := m.countAll() - limitNum
	countToRelease := min(excess, defaultBatchReleaseCount)
	if countToRelease <= 0 {
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
		countToRelease--
		return countToRelease > 0
	})
}

// login 批量登录空闲机器人，尝试进入桌子
func (m *Manager) login() {
	if !m.conf.Robot.Open {
		return
	}
	tables := m.repo.GetTableList()
	if len(tables) == 0 {
		return
	}

	count := 0
	m.free.Range(func(_, val any) bool {
		if count >= defaultBatchLoginCount {
			return false
		}
		p, ok := val.(*player.Player)
		if !ok || p.GetTableID() > 0 {
			return true
		}
		if err := table.CheckRoomLimit(p, m.conf.Game); err != nil {
			return true
		}

		metaVal, _ := m.meta.LoadOrStore(p.GetPlayerID(), &MetaData{})
		meta := metaVal.(*MetaData)
		if tb := meta.TryEnterTable(p, tables); tb != nil {
			m.free.Delete(p.GetPlayerID())
			count++
		}
		return true
	})
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
		m.meta.Delete(uid)
		return false
	}
	// 已经在空闲池
	if _, alreadyFree := m.free.Load(uid); alreadyFree {
		return true
	}

	m.Reset(p)
	m.free.Store(uid, p)
	return true
}

// Reset 重置机器人状态（比如金额）
func (m *Manager) Reset(p *player.Player) {
	m.updateMoney(p)
}

// updateMoney 根据配置更新机器人金额，保持金额在合理范围内
func (m *Manager) updateMoney(p *player.Player) {
	if p.GetTableID() > 0 {
		return
	}
	minMoney := max(m.conf.Robot.MinMoney, m.conf.Game.MinMoney)
	maxMoney := min(m.conf.Robot.MaxMoney, m.conf.Game.MaxMoney)
	money := p.GetBaseData().Money
	if money < minMoney || money > maxMoney {
		p.GetBaseData().Money = float64(int64(ext.RandFloat(minMoney, maxMoney)))
	}
}

// Counter 返回当前机器人总数、空闲数和游戏中数量
func (m *Manager) Counter() (all, free, gaming int32) {
	if !m.conf.Robot.Open || m.conf.Robot.Num <= 0 {
		return
	}
	all = m.countAll()
	free = m.countFree()
	return all, free, all - free
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
