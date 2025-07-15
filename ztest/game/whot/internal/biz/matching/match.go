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

// entry 堆中元素，封装玩家及超时截止时间
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

// Pool 匹配池
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
	// 暂无实现，如有需要可实现定时器停止等清理逻辑
}

// Add 添加玩家入队，忽略已存在玩家
func (p *Pool) Add(pl *player.Player) {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := pl.GetPlayerID()
	if _, exists := p.entryMap[id]; exists {
		return
	}
	p.insert(pl, randomTimeout(p.cfg.MinTimeoutMs, p.cfg.MaxTimeoutMs))
}

// Remove 移除玩家出队
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

// insert 私有，必须持锁
func (p *Pool) insert(pl *player.Player, deadline time.Time) {
	e := &entry{player: pl, deadline: deadline}
	heap.Push(&p.heap, e)
	p.entryMap[pl.GetPlayerID()] = e
}

// runMatchCycle 周期执行匹配任务
func (p *Pool) runMatchCycle() {
	p.mu.Lock()
	defer p.mu.Unlock()

	tables := p.repo.EmptyTables()
	if len(tables) == 0 {
		return
	}

	expired := p.popExpired()
	if len(expired) == 0 {
		return
	}

	// 移除机器人补充逻辑，完全在batchMatch中处理
	p.batchMatch(tables, expired)
}

// popExpired 弹出所有截止时间已到的玩家
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

// requeue 玩家重新入队，带随机延迟避免热点
func (p *Pool) requeue(players []*player.Player) {
	now := time.Now()
	for _, pl := range players {
		id := pl.GetPlayerID()
		if _, exists := p.entryMap[id]; !exists {
			p.insert(pl, now.Add(time.Millisecond*time.Duration(ext.RandInt(100, 1000))))
		}
	}
}

// batchMatch 批量匹配玩家和空桌，异步提交任务
// batchMatch 批量匹配玩家和空桌
func (p *Pool) batchMatch(tables []*table.Table, realPlayers []*player.Player) {
	loop := p.repo.GetLoop()

	tableIdx := 0
	playerIdx := 0
	for tableIdx < len(tables) && playerIdx < len(realPlayers) {
		// 随机决定组大小
		groupSize := ext.RandIntInclusive(p.cfg.MinGroupSize, p.cfg.MaxGroupSize)
		end := playerIdx + groupSize
		if end > len(realPlayers) {
			end = len(realPlayers)
		}

		// 当前组真实玩家
		groupRealPlayers := realPlayers[playerIdx:end]
		botNeeded := groupSize - len(groupRealPlayers)
		var bots []*player.Player

		// 需要补充机器人
		if botNeeded > 0 {
			bots = p.repo.AcquireBots(botNeeded)
			if len(bots) < botNeeded {
				// 机器人不足：释放已获取的，重入队当前组真实玩家
				p.repo.ReleaseBots(bots)
				p.requeue(groupRealPlayers)
				playerIdx = end // 跳过当前组
				break
			}
		}

		// 构建玩家组(真实玩家+机器人)
		group := make([]*player.Player, 0, len(groupRealPlayers)+len(bots))
		group = append(group, groupRealPlayers...)
		group = append(group, bots...)

		// 创建任务(记录真实玩家和机器人)
		task := &matchTask{
			table: tables[tableIdx],
			group: group,
			real:  groupRealPlayers, // 明确记录真实玩家
			bots:  bots,             // 明确记录机器人
			pool:  p,
		}

		loop.Post(func() {
			task.Run()
		})

		playerIdx = end
		tableIdx++
	}

	// 剩余真实玩家重入队
	if playerIdx < len(realPlayers) {
		p.requeue(realPlayers[playerIdx:])
	}
}

type matchTask struct {
	table *table.Table
	group []*player.Player // 全部玩家(真实+机器人)
	real  []*player.Player // 仅真实玩家
	bots  []*player.Player // 仅机器人
	pool  *Pool
}

func (t *matchTask) Run() {
	defer ext.RecoverFromError(nil)

	ok := t.table.JoinGroup(t.group)
	if !ok {
		// 入桌失败：释放机器人+真实玩家重入队
		t.pool.repo.ReleaseBots(t.bots)
		t.pool.requeue(t.real)
	}
}

// randomTimeout 生成随机超时时间点
func randomTimeout(minMs, maxMs int64) time.Time {
	ms := ext.RandIntInclusive(minMs, maxMs)
	return time.Now().Add(time.Duration(ms) * time.Millisecond)
}
