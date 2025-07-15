package matching

import (
	"container/heap"
	"sync"
	"time"

	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/table"
)

type Repo interface {
	EmptyTables() []*table.Table
	AcquireBots(n int) []*player.Player
	ReleaseBots([]*player.Player)
}

type Config struct {
	MinTimeoutMs  int64         // 匹配等待下限
	MaxTimeoutMs  int64         // 匹配等待上限
	MinGroupSize  int           // 最小组队人数
	MaxGroupSize  int           // 最大组队人数
	CheckInterval time.Duration // 匹配检查周期
}

// ---------- 堆结构实现 ----------
type entry struct {
	player   *player.Player
	deadline time.Time
	index    int
}

type entryHeap []*entry

func (h entryHeap) Len() int           { return len(h) }
func (h entryHeap) Less(i, j int) bool { return h[i].deadline.Before(h[j].deadline) }
func (h entryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
}

func (h *entryHeap) Push(x any) {
	e := x.(*entry)
	e.index = len(*h)
	*h = append(*h, e)
}

func (h *entryHeap) Pop() any {
	old := *h
	n := len(old)
	e := old[n-1]
	e.index = -1
	*h = old[:n-1]
	return e
}

// Pool 负责玩家匹配调度，管理玩家加入、超时匹配、补充机器人等
type Pool struct {
	repo     Repo
	cfg      *Config
	mu       sync.Mutex
	heap     entryHeap
	entryMap map[int64]*entry
	ticker   *time.Ticker
	stop     chan struct{}
}

func New(cfg *Config, repo Repo) *Pool {
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 500 * time.Millisecond
	}
	return &Pool{
		repo:     repo,
		cfg:      cfg,
		heap:     make(entryHeap, 0),
		entryMap: make(map[int64]*entry),
		ticker:   time.NewTicker(cfg.CheckInterval),
		stop:     make(chan struct{}),
	}
}

func (p *Pool) Start() {
	go func() {
		defer ext.RecoverFromError(func(e any) {
			p.Start()
		})
		for {
			select {
			case <-p.stop:
				return
			case <-p.ticker.C:
				p.runMatchCycle()
			}
		}
	}()
}

func (p *Pool) Stop() {
	close(p.stop)
	p.ticker.Stop()
}

func (p *Pool) Add(pl *player.Player) {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := pl.GetPlayerID()
	if _, exists := p.entryMap[id]; exists {
		return
	}
	p.insert(pl, randomTimeout(p.cfg.MinTimeoutMs, p.cfg.MaxTimeoutMs))
}

func (p *Pool) Remove(playerID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if e, ok := p.entryMap[playerID]; ok {
		if e.index >= 0 {
			heap.Remove(&p.heap, e.index)
		}
		delete(p.entryMap, playerID)
	}
}

func (p *Pool) runMatchCycle() {
	p.mu.Lock()
	defer p.mu.Unlock()

	group := p.popExpired()
	if len(group) == 0 {
		return
	}

	if len(group) < p.cfg.MinGroupSize {
		group = p.fillFromHeap(group, p.cfg.MinGroupSize-len(group))
	}

	p.batchMatch(group)
}

func (p *Pool) popExpired() []*player.Player {
	now := time.Now()
	var result []*player.Player

	for p.heap.Len() > 0 && p.heap[0].deadline.Before(now) {
		e := heap.Pop(&p.heap).(*entry)
		delete(p.entryMap, e.player.GetPlayerID())
		result = append(result, e.player)
	}
	return result
}

func (p *Pool) fillFromHeap(group []*player.Player, needed int) []*player.Player {
	for i := 0; i < needed && p.heap.Len() > 0; i++ {
		e := heap.Pop(&p.heap).(*entry)
		delete(p.entryMap, e.player.GetPlayerID())
		group = append(group, e.player)
	}
	return group
}

func (p *Pool) batchMatch(players []*player.Player) {
	for len(players) > 0 {
		n := ext.RandIntInclusive(p.cfg.MinGroupSize, p.cfg.MaxGroupSize)
		if n > len(players) {
			n = len(players)
		}

		group := players[:n]
		players = players[n:]

		matched := p.tryMatchGroup(group)
		if matched < len(group) {
			p.requeue(group[matched:])
		}
	}
}

func (p *Pool) tryMatchGroup(group []*player.Player) int {
	var bots []*player.Player
	if len(group) < p.cfg.MinGroupSize {
		missing := p.cfg.MinGroupSize - len(group)
		bots = p.repo.AcquireBots(missing)
		group = append(group, bots...)
	}

	if len(group) < p.cfg.MinGroupSize {
		p.repo.ReleaseBots(bots)
		return 0
	}

	if len(group) > p.cfg.MaxGroupSize {
		group = group[:p.cfg.MaxGroupSize]
	}

	for _, t := range p.repo.EmptyTables() {
		// 使用异步任务池 todo
		if t.JoinGroup(group) {
			return len(group)
		}
	}

	p.repo.ReleaseBots(bots)
	return 0
}

func (p *Pool) requeue(players []*player.Player) {
	now := time.Now()
	for _, pl := range players {
		if _, exists := p.entryMap[pl.GetPlayerID()]; !exists {
			p.insert(pl, now.Add(time.Millisecond*time.Duration(ext.RandInt(100, 1000))))
		}
	}
}

func (p *Pool) insert(pl *player.Player, deadline time.Time) {
	e := &entry{player: pl, deadline: deadline}
	heap.Push(&p.heap, e)
	p.entryMap[pl.GetPlayerID()] = e
}

func randomTimeout(minMs, maxMs int64) time.Time {
	ms := ext.RandIntInclusive(minMs, maxMs)
	return time.Now().Add(time.Duration(ms) * time.Millisecond)
}
