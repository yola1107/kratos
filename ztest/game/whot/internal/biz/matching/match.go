package matching

import (
	"errors"
	"math/rand"
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
	Player   *player.Player
	Deadline time.Time // 超时截止时间，加入匹配时随机生成
}

type MatchPool struct {
	mu      sync.Mutex
	waiting map[int64]*MatchEntry // 玩家ID -> 匹配信息
	config  MatchConfig
	tables  []*table.Table
	ticker  *time.Ticker
	stop    chan struct{}

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
		waiting: make(map[int64]*MatchEntry),
		tables:  tables,
		config:  cfg,
		ticker:  time.NewTicker(cfg.CheckInterval),
		stop:    make(chan struct{}),
	}, nil
}

func (p *MatchPool) Start() {
	go p.run()
}

func (p *MatchPool) Close() {
	close(p.stop)
	p.ticker.Stop()
}

// JoinMatch 玩家加入匹配，设置随机超时deadline
func (p *MatchPool) JoinMatch(pl *player.Player) {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := pl.GetPlayerID()
	if _, exists := p.waiting[id]; exists {
		return // 已经在匹配中，忽略
	}

	// 生成[minTimeoutSec, maxTimeoutSec]范围内的随机超时秒数
	timeoutSec := p.config.MinTimeoutSec
	if p.config.MaxTimeoutSec > p.config.MinTimeoutSec {
		delta := p.config.MaxTimeoutSec - p.config.MinTimeoutSec + 1
		timeoutSec = p.config.MinTimeoutSec + int32(rand.Intn(int(delta)))
	}

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)

	p.waiting[id] = &MatchEntry{
		Player:   pl,
		Deadline: deadline,
	}
	p.joined.Add(1)
}

// CancelMatch 取消匹配，立即移除玩家
func (p *MatchPool) CancelMatch(playerID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.waiting[playerID]; !ok {
		return
	}
	delete(p.waiting, playerID)
	p.canceled.Add(1)
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

// tryMatch 尝试匹配玩家
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

	// 把 map 转成切片并过滤无效玩家（冗余防护）
	entries := make([]*MatchEntry, 0, len(p.waiting))
	for _, entry := range p.waiting {
		if entry.Player != nil {
			entries = append(entries, entry)
		}
	}
	p.mu.Unlock()

	// 按deadline排序，先超时的靠前
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Deadline.Before(entries[j].Deadline)
	})

	// 超时玩家和正常玩家分离
	timedOut, normal := p.splitByTimeout(entries, now)
	p.timeouts.Add(int64(len(timedOut)))

	// 优先匹配超时玩家（组最小为2）
	matchedGroups := make([]matchGroup, 0)
	matched, idleTables := p.buildGroups(timedOut, idleTables, p.config.MinGroupSize)
	matchedGroups = append(matchedGroups, matched...)

	// 其次匹配正常玩家（组最小为 MinGroupSize）
	if len(idleTables) > 0 {
		normalMatched, _ := p.buildGroups(normal, idleTables, p.config.MinGroupSize)
		matchedGroups = append(matchedGroups, normalMatched...)
	}

	if len(matchedGroups) == 0 {
		return
	}

	p.commitMatchedGroups(matchedGroups)
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

func (p *MatchPool) splitByTimeout(entries []*MatchEntry, now time.Time) (timedOut, normal []*MatchEntry) {
	for _, e := range entries {
		if now.After(e.Deadline) {
			timedOut = append(timedOut, e)
		} else {
			normal = append(normal, e)
		}
	}
	return
}

func (p *MatchPool) buildGroups(entries []*MatchEntry, tables []*table.Table, minGroup int) (groups []matchGroup, remainTables []*table.Table) {
	if len(entries) < minGroup || len(tables) == 0 {
		return nil, tables
	}

	maxGroup := p.config.MaxGroupSize
	idx := 0
	tableIdx := 0
	for idx < len(entries) && tableIdx < len(tables) {
		remain := len(entries) - idx
		groupSize := maxGroup
		if remain < groupSize {
			groupSize = remain
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

	// 从等待map中移除匹配成功玩家
	for id := range removedIDs {
		delete(p.waiting, id)
	}
}

func (p *MatchPool) Stats() (joined, matched, canceled, timeouts int64) {
	return p.joined.Load(), p.matched.Load(), p.canceled.Load(), p.timeouts.Load()
}
