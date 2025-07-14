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
	mu        sync.RWMutex
	waiting   []*MatchEntry      // 等待匹配的玩家
	playerMap map[int64]struct{} // 玩家ID映射，用于判重
	tables    []*table.Table     // 所有桌子
	config    MatchConfig
	ticker    *time.Ticker
	stop      chan struct{}

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

	return &MatchPool{
		tables:    tables,
		config:    cfg,
		ticker:    time.NewTicker(cfg.CheckInterval),
		stop:      make(chan struct{}),
		playerMap: make(map[int64]struct{}),
	}, nil
}

// Start 启动匹配池
func (p *MatchPool) Start() { go p.run() }

// Close 停止匹配池
func (p *MatchPool) Close() {
	close(p.stop)
	p.ticker.Stop()
}

// Join 玩家加入匹配（带判重）
func (p *MatchPool) Join(pl *player.Player) {
	playerID := pl.GetPlayerID()

	p.mu.Lock()
	defer p.mu.Unlock()

	// 判重检查
	if _, exists := p.playerMap[playerID]; exists {
		return
	}

	// 添加玩家
	p.waiting = append(p.waiting, &MatchEntry{
		Player:    pl,
		EnterTime: time.Now(),
	})
	p.playerMap[playerID] = struct{}{}
	p.joined.Add(1)
}

// CancelMatch 取消匹配
func (p *MatchPool) CancelMatch(playerID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer p.canceled.Add(1)

	// 从映射中移除
	delete(p.playerMap, playerID)

	// 从等待列表中移除
	for i, entry := range p.waiting {
		if entry.Player.GetPlayerID() == playerID {
			// 高效移除：用最后一个元素替换当前位置
			p.waiting[i] = p.waiting[len(p.waiting)-1]
			p.waiting = p.waiting[:len(p.waiting)-1]
			return
		}
	}
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
	now := time.Now()

	// 1. 获取状态快照
	p.mu.RLock()
	if len(p.waiting) == 0 {
		p.mu.RUnlock()
		return
	}

	// 直接使用原始切片引用（只读操作安全）
	waiting := p.waiting
	idleTables := p.getIdleTables()
	p.mu.RUnlock()

	if len(idleTables) == 0 {
		return
	}

	// 2. 按进入时间排序（原地排序）
	sort.Slice(waiting, func(i, j int) bool {
		return waiting[i].EnterTime.Before(waiting[j].EnterTime)
	})

	// 3. 批量匹配玩家
	matchedGroups, remaining := p.batchMatch(waiting, idleTables, now)

	// 4. 分配匹配成功的组到桌子并移除玩家ID
	removedPlayerIDs := make(map[int64]struct{})
	for _, group := range matchedGroups {
		if group.table.JoinGroup(group.players) == nil {
			p.matched.Add(int64(len(group.players)))

			// 收集已匹配玩家ID
			for _, pl := range group.players {
				removedPlayerIDs[pl.GetPlayerID()] = struct{}{}
			}
		}
	}

	// 5. 更新等待池并移除已匹配玩家ID
	p.mu.Lock()
	defer p.mu.Unlock()

	// 移除已匹配玩家ID
	for playerID := range removedPlayerIDs {
		delete(p.playerMap, playerID)
	}

	// 直接使用剩余玩家切片
	p.waiting = remaining
}

// getIdleTables 获取当前空闲桌子（避免切片复制）
func (p *MatchPool) getIdleTables() []*table.Table {
	idleTables := make([]*table.Table, 0, len(p.tables))
	for _, tbl := range p.tables {
		if tbl.IsIdle() {
			idleTables = append(idleTables, tbl)
		}
	}
	return idleTables
}

// matchGroup 匹配组结构
type matchGroup struct {
	table   *table.Table
	players []*player.Player
}

// batchMatch 批量匹配玩家（零复制优化）
func (p *MatchPool) batchMatch(entries []*MatchEntry, availableTables []*table.Table, now time.Time) ([]matchGroup, []*MatchEntry) {
	var (
		matchedGroups []matchGroup
		remaining     []*MatchEntry
	)

	timeoutThreshold := time.Duration(p.config.MinTimeoutSec) * time.Second
	tableIdx := 0
	currentIndex := 0

	// 按顺序处理玩家
	for currentIndex < len(entries) && tableIdx < len(availableTables) {
		entry := entries[currentIndex]

		// 检查是否超时（只处理超时玩家）
		if now.Sub(entry.EnterTime) < timeoutThreshold {
			remaining = append(remaining, entry)
			currentIndex++
			continue
		}

		p.timeouts.Add(1)

		// 使用现有切片引用构建玩家组
		currentTable := availableTables[tableIdx]
		groupStart := currentIndex
		groupEnd := groupStart

		// 确定组边界
		for j := currentIndex; j < len(entries) && (j-groupStart) < p.config.MaxGroupSize; j++ {
			// 只包含超时玩家
			if now.Sub(entries[j].EnterTime) < timeoutThreshold {
				break
			}
			groupEnd = j + 1
		}

		// 检查组大小
		groupSize := groupEnd - groupStart
		if groupSize >= p.config.MinGroupSize {
			// 直接使用原始切片构建玩家组（避免复制）
			players := make([]*player.Player, 0, groupSize)
			for j := groupStart; j < groupEnd; j++ {
				players = append(players, entries[j].Player)
			}

			// 记录匹配组
			matchedGroups = append(matchedGroups, matchGroup{
				table:   currentTable,
				players: players,
			})

			// 移动到下一张桌子
			tableIdx++
			currentIndex = groupEnd
		} else {
			// 添加未匹配的玩家到剩余列表
			for j := groupStart; j < groupEnd; j++ {
				remaining = append(remaining, entries[j])
			}
			currentIndex = groupEnd
		}
	}

	// 添加剩余未处理的玩家到剩余列表
	for i := currentIndex; i < len(entries); i++ {
		remaining = append(remaining, entries[i])
	}

	return matchedGroups, remaining
}

// Stats 获取统计信息
func (p *MatchPool) Stats() (joined, matched, canceled, timeouts int64) {
	return p.joined.Load(), p.matched.Load(), p.canceled.Load(), p.timeouts.Load()
}
