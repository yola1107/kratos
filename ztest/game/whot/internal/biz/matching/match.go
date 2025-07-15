package matching

import (
	"container/heap"
	"sync"
	"time"

	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/table"
)

// Repo 抽象接口
type Repo interface {
	GetTableList() []*table.Table
	AcquireIdleAIs(n int) []*player.Player
}

type MatchConfig struct {
	MinTimeoutMs  int64         // 最小匹配超时时间（毫秒）
	MaxTimeoutMs  int64         // 最大匹配超时时间（毫秒）
	MinGroupSize  int           // 每组最少玩家数
	MaxGroupSize  int           // 每组最多玩家数
	CheckInterval time.Duration // 匹配检查间隔
}

type MatchPool struct {
	repo     Repo
	cfg      *MatchConfig
	mu       sync.Mutex
	heap     matchHeap
	entryMap map[int64]*heapEntry
	ticker   *time.Ticker
	stop     chan struct{}
}

func NewMatchPool(cfg *MatchConfig, repo Repo) *MatchPool {
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 500 * time.Millisecond
	}
	return &MatchPool{
		repo:     repo,
		cfg:      cfg,
		heap:     make(matchHeap, 0),
		entryMap: make(map[int64]*heapEntry),
		ticker:   time.NewTicker(cfg.CheckInterval),
		stop:     make(chan struct{}),
	}
}

func (p *MatchPool) Start() {
	go p.run()
}

func (p *MatchPool) Stop() {
	close(p.stop)
	p.ticker.Stop()
}

func (p *MatchPool) Add(pl *player.Player) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := pl.GetPlayerID()
	if _, exists := p.entryMap[id]; exists {
		return
	}
	now := time.Now()
	timeout := ext.RandIntInclusive(p.cfg.MinTimeoutMs, p.cfg.MaxTimeoutMs)

	deadline := now.Add(time.Duration(timeout) * time.Millisecond)
	e := &heapEntry{player: pl, deadline: deadline}
	heap.Push(&p.heap, e)
	p.entryMap[id] = e
}

func (p *MatchPool) Remove(playerID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.entryMap[playerID]; ok {
		heap.Remove(&p.heap, e.index)
		delete(p.entryMap, playerID)
	}
}

func (p *MatchPool) run() {
	for {
		select {
		case <-p.stop:
			return
		case <-p.ticker.C:
			p.match()
		}
	}
}

func (p *MatchPool) match() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	var timeoutPlayers []*player.Player

	// 1. 取出所有超时玩家
	for p.heap.Len() > 0 && p.heap[0].deadline.Before(now) {
		e := heap.Pop(&p.heap).(*heapEntry)
		delete(p.entryMap, e.player.GetPlayerID())
		timeoutPlayers = append(timeoutPlayers, e.player)
	}

	// 2. 按组分批处理超时玩家
	for len(timeoutPlayers) > 0 {
		groupSize := p.cfg.MaxGroupSize
		if len(timeoutPlayers) < groupSize {
			groupSize = len(timeoutPlayers)
		}
		group := timeoutPlayers[:groupSize]
		timeoutPlayers = timeoutPlayers[groupSize:]

		if len(group) >= p.cfg.MinGroupSize {
			p.tryEnterTable(group)
		} else {
			// 不足最小组大小，尝试补 AI
			missing := p.cfg.MinGroupSize - len(group)
			ais := p.repo.AcquireIdleAIs(missing)
			group = append(group, ais...)
			if len(group) >= p.cfg.MinGroupSize {
				p.tryEnterTable(group)
			}
			// 如果补完仍不足，就不管，等下一轮重新入 heap（可选）
		}
	}

	// 3. 如果剩余 heap 中玩家数量已满足构建新组，构建一组尝试入桌
	if len(p.heap) >= p.cfg.MinGroupSize {
		var all []*player.Player
		for _, e := range p.heap {
			all = append(all, e.player)
		}
		p.heap = p.heap[:0]
		p.entryMap = make(map[int64]*heapEntry)

		if len(all) > p.cfg.MaxGroupSize {
			all = all[:p.cfg.MaxGroupSize]
		}
		p.tryEnterTable(all)
	}
}

func (p *MatchPool) tryEnterTable(players []*player.Player) {
	if len(players) == 0 {
		return
	}
	if len(players) < p.cfg.MinGroupSize {
		missing := p.cfg.MinGroupSize - len(players)
		ais := p.repo.AcquireIdleAIs(missing)
		players = append(players, ais...)
		if len(players) < p.cfg.MinGroupSize {
			return // 补完仍不够
		}
	}
	if len(players) > p.cfg.MaxGroupSize {
		players = players[:p.cfg.MaxGroupSize]
	}
	for _, t := range p.repo.GetTableList() {
		if t.JoinGroup(players) {
			return
		}
	}
}

// ----------------------- heap ----------------------------

type heapEntry struct {
	player   *player.Player
	deadline time.Time
	index    int
}

type matchHeap []*heapEntry

func (h matchHeap) Len() int           { return len(h) }
func (h matchHeap) Less(i, j int) bool { return h[i].deadline.Before(h[j].deadline) }
func (h matchHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *matchHeap) Push(x any) {
	e := x.(*heapEntry)
	e.index = len(*h)
	*h = append(*h, e)
}

func (h *matchHeap) Pop() any {
	old := *h
	n := len(old)
	e := old[n-1]
	e.index = -1
	*h = old[:n-1]
	return e
}
