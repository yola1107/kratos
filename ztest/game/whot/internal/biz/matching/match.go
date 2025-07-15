package matching

import (
	"container/heap"
	"sync"
	"time"

	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/table"
)

// MatchPool 负责玩家匹配调度，管理玩家加入、超时匹配、补充机器人等
type MatchPool struct {
	repo     Repo
	cfg      *MatchConfig
	mu       sync.Mutex
	heap     matchHeap            // 小顶堆，按deadline排序
	entryMap map[int64]*heapEntry // playerID到堆元素的映射，便于快速删除
	ticker   *time.Ticker
	stop     chan struct{}
}

// MatchConfig 匹配配置参数
type MatchConfig struct {
	MinTimeoutMs  int64         // 最小超时毫秒数，随机超时范围下限
	MaxTimeoutMs  int64         // 最大超时毫秒数，随机超时范围上限
	MinGroupSize  int           // 最小匹配组人数
	MaxGroupSize  int           // 最大匹配组人数
	CheckInterval time.Duration // 匹配调度定时器间隔
}

// Repo 匹配需要的依赖接口，获取桌子列表和机器人
type Repo interface {
	GetTableList() []*table.Table
	AcquireIdleAIs(n int) []*player.Player
}

// NewMatchPool 创建匹配池实例
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

// Start 启动匹配调度循环
func (p *MatchPool) Start() {
	go func() {
		for {
			select {
			case <-p.stop:
				return
			case <-p.ticker.C:
				p.doMatch()
			}
		}
	}()
}

// Stop 停止匹配调度
func (p *MatchPool) Stop() {
	close(p.stop)
	p.ticker.Stop()
}

// Add 将玩家加入匹配池，设置随机超时截止时间
func (p *MatchPool) Add(pl *player.Player) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.entryMap[pl.GetPlayerID()]; exists {
		// 已存在，忽略重复添加
		return
	}

	// 生成随机超时时间
	timeoutMs := ext.RandIntInclusive(p.cfg.MinTimeoutMs, p.cfg.MaxTimeoutMs)
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)

	e := &heapEntry{player: pl, deadline: deadline}
	heap.Push(&p.heap, e)
	p.entryMap[pl.GetPlayerID()] = e
}

// Remove 从匹配池移除玩家
func (p *MatchPool) Remove(playerID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if e, ok := p.entryMap[playerID]; ok {
		heap.Remove(&p.heap, e.index)
		delete(p.entryMap, playerID)
	}
}

// doMatch 匹配调度，定时触发
func (p *MatchPool) doMatch() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	var expired []*player.Player

	// 1. 找出超时玩家，弹出并尝试匹配
	for p.heap.Len() > 0 && p.heap[0].deadline.Before(now) {
		e := heap.Pop(&p.heap).(*heapEntry)
		delete(p.entryMap, e.player.GetPlayerID())
		expired = append(expired, e.player)
	}

	// 2. 如果超时玩家人数不足最小组人数，尝试从非超时玩家中补充
	if len(expired) > 0 && len(expired) < p.cfg.MinGroupSize && p.heap.Len() > 0 {
		needed := p.cfg.MinGroupSize - len(expired)
		var nonExpired []*heapEntry

		// 从堆中弹出needed个非超时玩家补充
		for i := 0; i < needed && p.heap.Len() > 0; i++ {
			e := heap.Pop(&p.heap).(*heapEntry)
			delete(p.entryMap, e.player.GetPlayerID())
			nonExpired = append(nonExpired, e)
		}

		// 合并非超时玩家
		for _, e := range nonExpired {
			expired = append(expired, e.player)
		}
	}

	// 3. 用批量匹配函数进行匹配，匹配失败的玩家会被延迟重新加入匹配池
	p.batchMatchWithRetry(expired)
}

// batchMatchWithRetry 批量匹配超时玩家，匹配失败的玩家延长等待时间重新加入匹配池
func (p *MatchPool) batchMatchWithRetry(players []*player.Player) {
	for len(players) > 0 {
		n := ext.RandIntInclusive(p.cfg.MinGroupSize, p.cfg.MaxGroupSize)
		if len(players) < n {
			n = len(players)
		}
		group := append([]*player.Player(nil), players[:n]...)
		players = players[n:]

		matched := p.tryMatch(group)
		if matched < len(group) {
			// 匹配失败的玩家重新加入池中，延长等待时间
			for _, pl := range group[matched:] {
				deadline := time.Now().Add(100 * time.Millisecond)
				e := &heapEntry{player: pl, deadline: deadline}
				heap.Push(&p.heap, e)
				p.entryMap[pl.GetPlayerID()] = e
			}
		}
	}
}

// tryMatch 尝试匹配一组玩家，返回成功匹配的玩家数量
func (p *MatchPool) tryMatch(players []*player.Player) int {
	if len(players) < p.cfg.MinGroupSize {
		// 不够人时补充机器人
		missing := p.cfg.MinGroupSize - len(players)
		players = append(players, p.repo.AcquireIdleAIs(missing)...)
	}
	if len(players) < p.cfg.MinGroupSize {
		// 仍不够，匹配失败
		return 0
	}
	if len(players) > p.cfg.MaxGroupSize {
		players = players[:p.cfg.MaxGroupSize]
	}

	// 遍历桌子尝试加入玩家组
	for _, t := range p.repo.GetTableList() {
		if t.JoinGroup(players) {
			return len(players)
		}
	}
	return 0
}

// ---------- 堆结构实现 ----------

type heapEntry struct {
	player   *player.Player
	deadline time.Time
	index    int // 在堆中的索引
}

type matchHeap []*heapEntry

func (h matchHeap) Len() int           { return len(h) }
func (h matchHeap) Less(i, j int) bool { return h[i].deadline.Before(h[j].deadline) }
func (h matchHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
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
