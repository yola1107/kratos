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
	mu        sync.Mutex
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

func (p *MatchPool) Start() { go p.run() }

func (p *MatchPool) Close() {
	close(p.stop)
	p.ticker.Stop()
}

func (p *MatchPool) Join(pl *player.Player) {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := pl.GetPlayerID()
	if _, exists := p.playerMap[id]; exists {
		return
	}
	p.playerMap[id] = struct{}{}
	p.waiting = append(p.waiting, &MatchEntry{Player: pl, EnterTime: time.Now()})
	p.joined.Add(1)
}

func (p *MatchPool) CancelMatch(playerID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.playerMap[playerID]; !ok {
		return
	}
	delete(p.playerMap, playerID)
	p.canceled.Add(1)

	// 立即从等待列表移除
	for i := 0; i < len(p.waiting); i++ {
		if p.waiting[i].Player.GetPlayerID() == playerID {
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
			p.tryMatch()
		case <-p.stop:
			return
		}
	}
}

func (p *MatchPool) tryMatch() {
	now := time.Now()
	idleTables := p.getIdleTables()
	if len(idleTables) == 0 {
		return
	}

	p.mu.Lock()
	if len(p.waiting) == 0 {
		p.mu.Unlock()
		return
	}

	// 快照 + 清理非法玩家
	validEntries := make([]*MatchEntry, 0, len(p.waiting))
	for _, entry := range p.waiting {
		if _, ok := p.playerMap[entry.Player.GetPlayerID()]; ok {
			validEntries = append(validEntries, entry)
		}
	}
	sort.Slice(validEntries, func(i, j int) bool {
		return validEntries[i].EnterTime.Before(validEntries[j].EnterTime)
	})
	p.mu.Unlock()

	// 匹配
	timedOut, normal := p.splitTimeout(validEntries, now)
	p.timeouts.Add(int64(len(timedOut)))

	matched := make([]matchGroup, 0)
	matched, idleTables = p.buildGroups(timedOut, idleTables, 2)
	if len(idleTables) > 0 {
		normalMatched, _ := p.buildGroups(normal, idleTables, p.config.MinGroupSize)
		matched = append(matched, normalMatched...)
	}

	// 提交匹配组
	if len(matched) > 0 {
		p.commitMatchedGroups(matched)
	}
}

type matchGroup struct {
	table   *table.Table
	players []*player.Player
}

func (p *MatchPool) getIdleTables() []*table.Table {
	idle := make([]*table.Table, 0)
	for _, t := range p.tables {
		if t.IsIdle() {
			idle = append(idle, t)
		}
	}
	return idle
}

func (p *MatchPool) splitTimeout(entries []*MatchEntry, now time.Time) (timedOut, normal []*MatchEntry) {
	threshold := time.Duration(p.config.MinTimeoutSec) * time.Second
	for _, entry := range entries {
		if now.Sub(entry.EnterTime) >= threshold {
			timedOut = append(timedOut, entry)
		} else {
			normal = append(normal, entry)
		}
	}
	return
}

func (p *MatchPool) buildGroups(entries []*MatchEntry, tables []*table.Table, minGroup int) (groups []matchGroup, remainingTables []*table.Table) {
	if len(entries) < minGroup || len(tables) == 0 {
		return nil, tables
	}

	maxGroup := p.config.MaxGroupSize
	idx := 0
	tableIdx := 0

	for idx < len(entries) && tableIdx < len(tables) {
		left := len(entries) - idx
		groupSize := maxGroup
		if left < groupSize {
			groupSize = left
		}
		if groupSize < minGroup {
			break
		}
		players := make([]*player.Player, groupSize)
		for i := 0; i < groupSize; i++ {
			players[i] = entries[idx+i].Player
		}
		groups = append(groups, matchGroup{
			table:   tables[tableIdx],
			players: players,
		})
		idx += groupSize
		tableIdx++
	}
	return groups, tables[tableIdx:]
}

func (p *MatchPool) commitMatchedGroups(groups []matchGroup) {
	removedIDs := make(map[int64]struct{})

	for _, g := range groups {
		if g.table.JoinGroup(g.players) == nil {
			for _, pl := range g.players {
				removedIDs[pl.GetPlayerID()] = struct{}{}
			}
			p.matched.Add(int64(len(g.players)))
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// 移除匹配成功玩家
	for id := range removedIDs {
		delete(p.playerMap, id)
	}
	filtered := p.waiting[:0]
	for _, entry := range p.waiting {
		if _, ok := p.playerMap[entry.Player.GetPlayerID()]; ok {
			filtered = append(filtered, entry)
		}
	}
	p.waiting = filtered

	p.shrinkWaiting()
}

func (p *MatchPool) shrinkWaiting() {
	const shrinkThreshold = 1024
	if cap(p.waiting) > len(p.waiting)*2 && cap(p.waiting) > shrinkThreshold {
		newList := make([]*MatchEntry, len(p.waiting))
		copy(newList, p.waiting)
		p.waiting = newList
	}
}

func (p *MatchPool) Stats() (joined, matched, canceled, timeouts int64) {
	return p.joined.Load(), p.matched.Load(), p.canceled.Load(), p.timeouts.Load()
}
