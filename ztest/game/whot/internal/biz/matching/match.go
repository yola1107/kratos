package matching

import (
	"container/heap"
	"sync"
	"time"

	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/table"
)

// Repo 提供外部资源接口
type Repo interface {
	GetLoop() work.ITaskLoop       // 异步任务执行器（线程池或协程池）
	GetTimer() work.ITaskScheduler // 定时器
	EmptyTables() []*table.Table   // 空闲桌子列表
	AcquireBots(n int) []*player.Player
	ReleaseBots([]*player.Player)
}

// Config 配置项
type Config struct {
	MinTimeoutMs  int64
	MaxTimeoutMs  int64
	MinGroupSize  int
	MaxGroupSize  int
	CheckInterval time.Duration
}

// entry 表示等待玩家及其超时时间，堆元素
type entry struct {
	player   *player.Player
	deadline time.Time
	index    int // 堆索引，方便移除
}

type entryHeap []*entry

func (h entryHeap) Len() int           { return len(h) }
func (h entryHeap) Less(i, j int) bool { return h[i].deadline.Before(h[j].deadline) }
func (h entryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *entryHeap) Push(x interface{}) {
	e := x.(*entry)
	e.index = len(*h)
	*h = append(*h, e)
}

func (h *entryHeap) Pop() interface{} {
	old := *h
	n := len(old)
	e := old[n-1]
	e.index = -1
	*h = old[:n-1]
	return e
}

// Pool 是匹配池，负责玩家入队、定时匹配、机器人补充等
type Pool struct {
	repo     Repo
	cfg      *Config
	mu       sync.Mutex
	heap     entryHeap
	entryMap map[int64]*entry // playerID -> entry，避免重复入队
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
	}
}

func (p *Pool) Start() {
	p.repo.GetTimer().ForeverNow(p.cfg.CheckInterval, p.runMatchCycle)
}

func (p *Pool) Stop() {
	// 这里可实现停止定时器逻辑，视work.ITaskScheduler接口而定
}

// Add 玩家入队，若已存在则忽略
func (p *Pool) Add(pl *player.Player) {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := pl.GetPlayerID()
	if _, exists := p.entryMap[id]; exists {
		return
	}
	p.insert(pl, randomTimeout(p.cfg.MinTimeoutMs, p.cfg.MaxTimeoutMs))
}

// Remove 从队列中移除玩家
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

// insert 内部插入，需持锁
func (p *Pool) insert(pl *player.Player, deadline time.Time) {
	e := &entry{player: pl, deadline: deadline}
	heap.Push(&p.heap, e)
	p.entryMap[pl.GetPlayerID()] = e
}

// runMatchCycle 定时执行匹配
func (p *Pool) runMatchCycle() {
	p.mu.Lock()
	expired := p.popExpired()
	if len(expired) < p.cfg.MinGroupSize {
		expired = p.fillFromHeap(expired, p.cfg.MinGroupSize-len(expired))
	}
	p.mu.Unlock()

	if len(expired) == 0 {
		return
	}

	// 这里异步批量匹配，避免锁阻塞任务执行
	p.batchMatch(expired)
}

// popExpired 弹出所有超时玩家
func (p *Pool) popExpired() []*player.Player {
	now := time.Now()
	var expired []*player.Player

	for p.heap.Len() > 0 && !p.heap[0].deadline.After(now) {
		e := heap.Pop(&p.heap).(*entry)
		delete(p.entryMap, e.player.GetPlayerID())
		expired = append(expired, e.player)
	}
	return expired
}

// fillFromHeap 补充玩家至指定数量
func (p *Pool) fillFromHeap(group []*player.Player, needed int) []*player.Player {
	for i := 0; i < needed && p.heap.Len() > 0; i++ {
		e := heap.Pop(&p.heap).(*entry)
		delete(p.entryMap, e.player.GetPlayerID())
		group = append(group, e.player)
	}
	return group
}

// matchTask 封装异步匹配任务，避免闭包引用陷阱
type matchTask struct {
	table *table.Table
	group []*player.Player
	bots  []*player.Player
	pool  *Pool
}

func (t *matchTask) Run() {
	ok := t.table.JoinGroup(t.group)
	if !ok {
		t.pool.repo.ReleaseBots(t.bots)
		t.pool.requeue(t.group)
	}
	// 成功则机器人自动管理，无需释放
}

// requeue 重新入队玩家
func (p *Pool) requeue(players []*player.Player) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for _, pl := range players {
		id := pl.GetPlayerID()
		if _, exists := p.entryMap[id]; !exists {
			p.insert(pl, now.Add(time.Millisecond*time.Duration(ext.RandInt(100, 1000))))
		}
	}
}

// batchMatch 根据空桌和玩家批量异步提交匹配任务
func (p *Pool) batchMatch(players []*player.Player) {
	tables := p.repo.EmptyTables()
	loop := p.repo.GetLoop()

	tableIdx, playerIdx := 0, 0
	for tableIdx < len(tables) && playerIdx < len(players) {
		n := ext.RandIntInclusive(p.cfg.MinGroupSize, p.cfg.MaxGroupSize)
		end := playerIdx + n
		if end > len(players) {
			end = len(players)
		}
		group := players[playerIdx:end]

		var bots []*player.Player
		if len(group) < p.cfg.MinGroupSize {
			bots = p.repo.AcquireBots(p.cfg.MinGroupSize - len(group))
			group = append(group, bots...)
		}
		if len(group) < p.cfg.MinGroupSize {
			p.repo.ReleaseBots(bots)
			break
		}
		if len(group) > p.cfg.MaxGroupSize {
			group = group[:p.cfg.MaxGroupSize]
		}

		groupCopy := make([]*player.Player, len(group))
		copy(groupCopy, group)

		task := &matchTask{
			table: tables[tableIdx],
			group: groupCopy,
			bots:  bots,
			pool:  p,
		}

		loop.Post(task.Run)

		playerIdx = end
		tableIdx++
	}

	// 剩余玩家重新入队
	if playerIdx < len(players) {
		p.requeue(players[playerIdx:])
	}
}

// randomTimeout 随机超时截止时间
func randomTimeout(minMs, maxMs int64) time.Time {
	ms := ext.RandIntInclusive(minMs, maxMs)
	return time.Now().Add(time.Duration(ms) * time.Millisecond)
}
