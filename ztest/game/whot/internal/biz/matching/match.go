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

type MatchConfig struct {
	MinTimeoutSec int32
	MaxTimeoutSec int32
	MinGroupSize  int
	MaxGroupSize  int
	CheckInterval time.Duration
}

type MatchEntry struct {
	Player    *player.Player
	EnterTime time.Time
}

type MatchPool struct {
	mu        sync.RWMutex
	waiting   []*MatchEntry
	playerMap map[int64]struct{}
	tables    []*table.Table
	config    MatchConfig
	ticker    *time.Ticker
	stop      chan struct{}

	joined   atomic.Int64
	matched  atomic.Int64
	canceled atomic.Int64
	timeouts atomic.Int64
}

func NewMatchPool(tables []*table.Table, cfg MatchConfig) (*MatchPool, error) {
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

func (p *MatchPool) Start() {
	go p.run()
}

func (p *MatchPool) Close() {
	close(p.stop)
	p.ticker.Stop()
}

func (p *MatchPool) Join(pl *player.Player) {
	playerID := pl.GetPlayerID()
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.playerMap[playerID]; exists {
		return
	}

	p.waiting = append(p.waiting, &MatchEntry{Player: pl, EnterTime: time.Now()})
	p.playerMap[playerID] = struct{}{}
	p.joined.Add(1)
}

func (p *MatchPool) CancelMatch(playerID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.canceled.Add(1)

	delete(p.playerMap, playerID)
	for i, entry := range p.waiting {
		if entry.Player.GetPlayerID() == playerID {
			// 快速删除，保持顺序无关紧要
			p.waiting[i] = p.waiting[len(p.waiting)-1]
			p.waiting = p.waiting[:len(p.waiting)-1]
			break
		}
	}
}

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

func (p *MatchPool) matchPlayers() {
	now := time.Now()

	p.mu.RLock()
	if len(p.waiting) == 0 {
		p.mu.RUnlock()
		return
	}
	waitingCopy := make([]*MatchEntry, len(p.waiting))
	copy(waitingCopy, p.waiting)
	p.mu.RUnlock()

	idleTables := p.getIdleTables()
	if len(idleTables) == 0 {
		return
	}

	sort.Slice(waitingCopy, func(i, j int) bool {
		return waitingCopy[i].EnterTime.Before(waitingCopy[j].EnterTime)
	})

	matchedGroups, remaining := p.batchMatch(waitingCopy, idleTables, now)

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, group := range matchedGroups {
		if group.table.JoinGroup(group.players) == nil {
			p.matched.Add(int64(len(group.players)))
			for _, pl := range group.players {
				delete(p.playerMap, pl.GetPlayerID())
			}
		} else {
			// 失败玩家回退，确保无丢失
			for _, pl := range group.players {
				remaining = append(remaining, &MatchEntry{Player: pl, EnterTime: now})
				p.playerMap[pl.GetPlayerID()] = struct{}{}
			}
		}
	}

	p.waiting = remaining
}

func (p *MatchPool) getIdleTables() []*table.Table {
	idle := make([]*table.Table, 0, len(p.tables))
	for _, t := range p.tables {
		if t.IsIdle() {
			idle = append(idle, t)
		}
	}
	return idle
}

type matchGroup struct {
	table   *table.Table
	players []*player.Player
}

// batchMatch 批量匹配玩家到可用桌子
// entries: 等待匹配的玩家条目（按进入时间排序）
// availableTables: 当前可用的桌子列表
// now: 当前时间，用于判断玩家等待超时
// 返回：
// - 成功匹配的组（包含桌子和玩家列表）
// - 未匹配的剩余玩家（继续等待）
func (p *MatchPool) batchMatch(entries []*MatchEntry, availableTables []*table.Table, now time.Time) ([]matchGroup, []*MatchEntry) {
	var (
		matchedGroups []matchGroup  // 匹配成功的玩家组
		remaining     []*MatchEntry // 剩余未匹配玩家
	)

	timeoutThreshold := time.Duration(p.config.MinTimeoutSec) * time.Second
	tableIdx := 0 // 桌子索引
	i := 0        // 玩家条目索引

	for i < len(entries) && tableIdx < len(availableTables) {
		entry := entries[i]

		// 玩家未达到超时门槛，放入剩余等待
		if now.Sub(entry.EnterTime) < timeoutThreshold {
			remaining = append(remaining, entry)
			i++
			continue
		}

		// 当前玩家已超时，尝试组队匹配
		p.timeouts.Add(1) // 统计超时次数

		groupStart := i
		groupEnd := groupStart

		// 扩大组队边界，所有玩家均超时，且组不超过最大人数
		for groupEnd < len(entries) &&
			(groupEnd-groupStart) < p.config.MaxGroupSize &&
			now.Sub(entries[groupEnd].EnterTime) >= timeoutThreshold {
			groupEnd++
		}

		groupSize := groupEnd - groupStart

		if groupSize >= p.config.MinGroupSize {
			// 匹配成功，提取玩家
			players := make([]*player.Player, 0, groupSize)
			for j := groupStart; j < groupEnd; j++ {
				players = append(players, entries[j].Player)
			}

			matchedGroups = append(matchedGroups, matchGroup{
				table:   availableTables[tableIdx],
				players: players,
			})

			tableIdx++   // 分配下一张桌子
			i = groupEnd // 跳过已匹配玩家
		} else {
			// 组队不够大，放入剩余等待
			for j := groupStart; j < groupEnd; j++ {
				remaining = append(remaining, entries[j])
			}
			i = groupEnd
		}
	}

	// 追加剩余未处理的玩家
	for ; i < len(entries); i++ {
		remaining = append(remaining, entries[i])
	}

	return matchedGroups, remaining
}

func (p *MatchPool) Stats() (joined, matched, canceled, timeouts int64) {
	return p.joined.Load(), p.matched.Load(), p.canceled.Load(), p.timeouts.Load()
}
