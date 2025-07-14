package matching

import (
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/table"
)

// MatchConfig 匹配配置
type MatchConfig struct {
	MinTimeoutSec int32         // 最短匹配等待时间（秒）
	MaxTimeoutSec int32         // 最长匹配等待时间（秒）
	MinGroupSize  int           // 每桌最少匹配人数
	MaxGroupSize  int           // 每桌最多匹配人数
	CheckInterval time.Duration // 检查间隔，默认100ms
}

// MatchEntry 匹配条目
type MatchEntry struct {
	Player    *player.Player
	EnterTime time.Time
}

// MatchPool 匹配池
type MatchPool struct {
	mu         sync.RWMutex
	waiting    []*MatchEntry  // 等待匹配的玩家
	idleTables []*table.Table // 空闲桌子列表
	config     MatchConfig
	ticker     *time.Ticker
	stop       chan struct{}

	// 统计
	joined   atomic.Int64
	matched  atomic.Int64
	canceled atomic.Int64
	timeouts atomic.Int64
}

// NewMatchPool 创建匹配池
func NewMatchPool(tables []*table.Table, cfg MatchConfig) (*MatchPool, error) {
	// 配置验证
	if cfg.MinTimeoutSec <= 0 || cfg.MaxTimeoutSec < cfg.MinTimeoutSec {
		return nil, errors.New("invalid timeout config")
	}
	if cfg.MinGroupSize <= 0 || cfg.MaxGroupSize < cfg.MinGroupSize {
		return nil, errors.New("invalid group size config")
	}
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 100 * time.Millisecond
	}

	// 初始化空闲桌子
	var idleTables []*table.Table
	for _, t := range tables {
		if t.IsIdle() {
			idleTables = append(idleTables, t)
		}
	}

	return &MatchPool{
		idleTables: idleTables,
		config:     cfg,
		ticker:     time.NewTicker(cfg.CheckInterval),
		stop:       make(chan struct{}),
	}, nil
}

// Start 启动匹配池
func (p *MatchPool) Start() { go p.run() }

// Close 停止匹配池
func (p *MatchPool) Close() {
	close(p.stop)
	p.ticker.Stop()
}

// Join 玩家加入匹配
func (p *MatchPool) Join(pl *player.Player) {
	p.mu.Lock()
	p.waiting = append(p.waiting, &MatchEntry{
		Player:    pl,
		EnterTime: time.Now(),
	})
	p.mu.Unlock()
	p.joined.Add(1)
}

// CancelMatch 取消匹配
func (p *MatchPool) CancelMatch(playerID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer p.canceled.Add(1)

	// 高效过滤
	newWaiting := p.waiting[:0]
	for _, entry := range p.waiting {
		if entry.Player.GetPlayerID() != playerID {
			newWaiting = append(newWaiting, entry)
		}
	}
	p.waiting = newWaiting
}

// run 匹配主循环
func (p *MatchPool) run() {
	for {
		select {
		case <-p.ticker.C:
			p.matchPlayers()
		case <-p.stop:
			return
		}
	}
}

// matchPlayers 核心匹配逻辑
func (p *MatchPool) matchPlayers() {
	// 1. 获取当前状态快照
	now := time.Now()

	p.mu.Lock()
	waiting := make([]*MatchEntry, len(p.waiting))
	copy(waiting, p.waiting)
	idleTables := make([]*table.Table, len(p.idleTables))
	copy(idleTables, p.idleTables)
	p.mu.Unlock()

	if len(waiting) == 0 || len(idleTables) == 0 {
		return
	}

	// 2. 按进入时间排序（最早优先）
	sort.Slice(waiting, func(i, j int) bool {
		return waiting[i].EnterTime.Before(waiting[j].EnterTime)
	})

	// 3. 批量匹配玩家
	var (
		remaining   []*MatchEntry      // 未匹配玩家
		matchedSets [][]*player.Player // 匹配成功的玩家组
		usedTables  []*table.Table     // 已使用的桌子
	)

	// 处理超时玩家
	timeoutThreshold := time.Duration(p.config.MinTimeoutSec) * time.Second
	var timedOutPlayers []*MatchEntry

	for _, entry := range waiting {
		if now.Sub(entry.EnterTime) >= timeoutThreshold {
			timedOutPlayers = append(timedOutPlayers, entry)
			p.timeouts.Add(1)
		} else {
			remaining = append(remaining, entry)
		}
	}

	// 4. 批量匹配超时玩家
	matchedSets, usedTables, remaining = p.batchMatch(timedOutPlayers, idleTables, remaining)

	// 5. 尝试匹配剩余玩家（即使未超时）
	if len(remaining) > 0 && len(idleTables) > len(usedTables) {
		availableTables := idleTables[len(usedTables):]
		newSets, newTables, newRemaining := p.batchMatch(remaining, availableTables, nil)

		matchedSets = append(matchedSets, newSets...)
		usedTables = append(usedTables, newTables...)
		remaining = newRemaining
	}

	// 6. 分配匹配成功的组到桌子
	for i, players := range matchedSets {
		table := usedTables[i]
		if table.JoinGroup(players) == nil {
			// 标记桌子已使用
			p.mu.Lock()
			// 从空闲桌子列表中移除
			for j, t := range p.idleTables {
				if t == table {
					p.idleTables = append(p.idleTables[:j], p.idleTables[j+1:]...)
					break
				}
			}
			p.mu.Unlock()
		}
		p.matched.Add(int64(len(players)))
	}

	// 7. 更新等待池
	p.mu.Lock()
	p.waiting = remaining
	p.mu.Unlock()
}

// batchMatch 批量匹配玩家
func (p *MatchPool) batchMatch(players []*MatchEntry, tables []*table.Table, existingRemaining []*MatchEntry) (
	matchedSets [][]*player.Player,
	usedTables []*table.Table,
	remaining []*MatchEntry) {

	if existingRemaining != nil {
		remaining = existingRemaining
	} else {
		remaining = make([]*MatchEntry, 0, len(players))
	}

	// 当前匹配组
	currentGroup := make([]*player.Player, 0, p.config.MaxGroupSize)

	for i := 0; i < len(players) && len(usedTables) < len(tables); {
		player := players[i]

		// 添加到当前组
		currentGroup = append(currentGroup, player.Player)

		// 检查是否达到最大组大小或没有更多玩家
		if len(currentGroup) == p.config.MaxGroupSize || i == len(players)-1 {
			// 检查是否满足最小组要求
			if len(currentGroup) >= p.config.MinGroupSize {
				// 分配新桌子
				table := tables[len(usedTables)]
				usedTables = append(usedTables, table)
				matchedSets = append(matchedSets, currentGroup)
				currentGroup = make([]*player.Player, 0, p.config.MaxGroupSize)
			} else {
				// 组太小，回退到等待池
				for _, p := range currentGroup {
					remaining = append(remaining, &MatchEntry{Player: p})
				}
				currentGroup = nil
			}
		}

		i++
	}

	// 处理未完成的组
	if len(currentGroup) > 0 {
		remaining = append(remaining, &MatchEntry{Player: currentGroup[0]})
		if len(currentGroup) > 1 {
			remaining = append(remaining, &MatchEntry{Player: currentGroup[1]})
		}
	}

	return matchedSets, usedTables, remaining
}

// Stats 获取统计信息
func (p *MatchPool) Stats() (joined, matched, canceled, timeouts int64) {
	return p.joined.Load(), p.matched.Load(), p.canceled.Load(), p.timeouts.Load()
}
