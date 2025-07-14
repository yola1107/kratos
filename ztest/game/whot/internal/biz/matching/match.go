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

	// 1. 判重：检查玩家是否已在匹配池
	if _, exists := p.playerMap[playerID]; exists {
		return
	}

	// 2. 添加玩家
	entry := &MatchEntry{
		Player:    pl,
		EnterTime: time.Now(),
	}
	p.waiting = append(p.waiting, entry)
	p.playerMap[playerID] = struct{}{}
	p.joined.Add(1)
}

// CancelMatch 取消匹配（优化重分配）
func (p *MatchPool) CancelMatch(playerID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer p.canceled.Add(1)

	// 1. 从映射中移除
	delete(p.playerMap, playerID)

	// 2. 高效移除（避免不必要的重分配）
	found := false
	for i, entry := range p.waiting {
		if entry.Player.GetPlayerID() == playerID {
			// 将最后一个元素移到当前位置
			if i < len(p.waiting)-1 {
				p.waiting[i] = p.waiting[len(p.waiting)-1]
			}
			// 缩短切片
			p.waiting = p.waiting[:len(p.waiting)-1]
			found = true
			break
		}
	}

	// 如果没有找到，不需要做任何操作
	if !found {
		return
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
	// 1. 获取当前状态快照
	now := time.Now()

	p.mu.RLock()
	// 复制等待玩家列表
	waiting := make([]*MatchEntry, len(p.waiting))
	copy(waiting, p.waiting)

	// 获取当前空闲桌子
	idleTables := p.getIdleTables()
	p.mu.RUnlock()

	if len(waiting) == 0 || len(idleTables) == 0 {
		return
	}

	// 2. 按进入时间排序（最早优先）
	sort.Slice(waiting, func(i, j int) bool {
		return waiting[i].EnterTime.Before(waiting[j].EnterTime)
	})

	// 3. 批量匹配玩家
	matchedGroups, remaining := p.batchMatch(waiting, idleTables, now)

	// 4. 分配匹配成功的组到桌子
	for _, group := range matchedGroups {
		if group.table.JoinGroup(group.players) == nil {
			p.matched.Add(int64(len(group.players)))
		}
	}

	// 5. 更新等待池（加锁更新）
	p.mu.Lock()
	p.waiting = remaining
	p.mu.Unlock()
}

// getIdleTables 获取当前空闲桌子（显式状态更新）
func (p *MatchPool) getIdleTables() []*table.Table {
	var idleTables []*table.Table
	for _, t := range p.tables {
		if t.IsIdle() {
			idleTables = append(idleTables, t)
		}
	}
	return idleTables
}

// 匹配组结构
type matchGroup struct {
	table   *table.Table
	players []*player.Player
}

// batchMatch 批量匹配玩家（重构版）
func (p *MatchPool) batchMatch(players []*MatchEntry, tables []*table.Table, now time.Time) ([]matchGroup, []*MatchEntry) {
	var (
		matchedGroups []matchGroup
		remaining     []*MatchEntry
	)

	timeoutThreshold := time.Duration(p.config.MinTimeoutSec) * time.Second
	tableIndex := 0 // 当前可用桌子的索引

	// 按顺序处理玩家
	for i := 0; i < len(players) && tableIndex < len(tables); {
		player := players[i]

		// 检查是否超时（只处理超时玩家）
		if now.Sub(player.EnterTime) < timeoutThreshold {
			remaining = append(remaining, player)
			i++
			continue
		}

		p.timeouts.Add(1)

		// 尝试组建一组玩家
		groupSize := 0
		groupPlayers := make([]*player.Player, 0, p.config.MaxGroupSize)
		table := tables[tableIndex]

		// 从当前玩家开始组建一组
		for j := i; j < len(players) && groupSize < p.config.MaxGroupSize; j++ {
			// 只包含超时玩家
			if now.Sub(players[j].EnterTime) < timeoutThreshold {
				break
			}

			groupPlayers = append(groupPlayers, players[j].Player)
			groupSize++

			// 达到最小组大小且有空闲桌子
			if groupSize >= p.config.MinGroupSize {
				// 记录匹配组
				matchedGroups = append(matchedGroups, matchGroup{
					table:   table,
					players: groupPlayers,
				})

				// 移动到下一张桌子
				tableIndex++

				// 跳过已匹配的玩家
				i = j + 1
				break
			}
		}

		// 如果组没有完成，将玩家放回剩余列表
		if groupSize < p.config.MinGroupSize {
			for k := i; k < i+groupSize; k++ {
				remaining = append(remaining, players[k])
			}
			i += groupSize
		}
	}

	// 添加未处理的玩家到剩余列表
	if tableIndex >= len(tables) {
		remaining = append(remaining, players[i:]...)
	}

	return matchedGroups, remaining
}

// Stats 获取统计信息
func (p *MatchPool) Stats() (joined, matched, canceled, timeouts int64) {
	return p.joined.Load(), p.matched.Load(), p.canceled.Load(), p.timeouts.Load()
}
