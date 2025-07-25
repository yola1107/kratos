package match

import (
	"container/heap"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
)

// ================= Match Levels =================

type Level int

const (
	LevelNewbie       Level = iota // 新手
	LevelNormal                    // 普通/充值用户
	LevelHighWithdraw              // 提现比高
	LevelRisk                      // 风控/黑户/点控
)

// ================= Pool =================

type Pool struct {
	buckets   map[int]*Bucket
	bucketsMu sync.RWMutex
	antsPool  *ants.Pool
}

func NewPool(levels []int, checkTick time.Duration, poolSize int) (*Pool, error) {
	pool, err := ants.NewPool(poolSize)
	if err != nil {
		return nil, err
	}
	sys := &Pool{
		buckets:  make(map[int]*Bucket),
		antsPool: pool,
	}
	for _, lvl := range levels {
		sys.buckets[lvl] = NewMatchBucket(lvl, pool, checkTick)
	}
	return sys, nil
}

func (s *Pool) Add(playerID int64, level int) {
	s.bucketsMu.RLock()
	bucket, ok := s.buckets[level]
	s.bucketsMu.RUnlock()
	if !ok {
		fmt.Printf("等级 %d 不存在\n", level)
		return
	}

	now := time.Now()
	matchTimeout := time.Duration(6+rand.Intn(7)) * time.Second // 6~12秒
	matchAt := now.Add(matchTimeout)
	deadAt := matchAt.Add(10 * time.Second)

	task := &Task{
		PlayerID: playerID,
		Level:    level,
		JoinAt:   now,
		MatchAt:  matchAt,
		DeadAt:   deadAt,
	}
	bucket.AddTask(task)

	fmt.Printf("玩家 %d 加入匹配 (等级: %d)，MatchAt: %s, DeadAt: %s\n",
		playerID, level, matchAt.Format("15:04:05"), deadAt.Format("15:04:05"))
}

func (s *Pool) Remove(task *Task) {
	s.bucketsMu.RLock()
	bucket, ok := s.buckets[task.Level]
	s.bucketsMu.RUnlock()
	if ok {
		bucket.CancelTask(task)
	}
}

func (s *Pool) Shutdown() {
	s.bucketsMu.Lock()
	for _, b := range s.buckets {
		b.Stop()
	}
	s.bucketsMu.Unlock()
	s.antsPool.Release()
}

// ================= Bucket =================

type Bucket struct {
	level     int
	heap      MinHeap
	heapMu    sync.Mutex
	antsPool  *ants.Pool
	checkTick time.Duration
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

func NewMatchBucket(level int, antsPool *ants.Pool, tick time.Duration) *Bucket {
	b := &Bucket{
		level:     level,
		heap:      make(MinHeap, 0),
		antsPool:  antsPool,
		checkTick: tick,
		stopCh:    make(chan struct{}),
	}
	heap.Init(&b.heap)
	b.wg.Add(1)
	go b.run()
	return b
}

func (b *Bucket) run() {
	defer b.wg.Done()
	ticker := time.NewTicker(b.checkTick)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			b.checkExpired()
		case <-b.stopCh:
			return
		}
	}
}

func (b *Bucket) Stop() {
	close(b.stopCh)
	b.wg.Wait()
}

func (b *Bucket) AddTask(task *Task) {
	b.heapMu.Lock()
	defer b.heapMu.Unlock()
	heap.Push(&b.heap, task)
}

func (b *Bucket) CancelTask(task *Task) {
	_ = b.antsPool.Submit(func() {
		task.Cancel()
	})
}

func (b *Bucket) checkExpired() {
	now := time.Now()
	var expired []*Task

	b.heapMu.Lock()
	for b.heap.Len() > 0 {
		task := b.heap[0]
		if task.IsCanceled() {
			heap.Pop(&b.heap)
			continue
		}
		if task.MatchAt.After(now) {
			break
		}
		expired = append(expired, heap.Pop(&b.heap).(*Task))
	}
	b.heapMu.Unlock()

	if len(expired) > 0 {
		_ = b.antsPool.Submit(func() {
			b.batchMatch(expired)
		})
	}
}

func (b *Bucket) batchMatch(tasks []*Task) {
	deskSize := 4
	for i := 0; i < len(tasks); i += deskSize {
		end := i + deskSize
		if end > len(tasks) {
			end = len(tasks)
		}
		group := tasks[i:end]
		tableType := b.getTableType()
		fmt.Printf("[Level %d][%s] 创建新桌，玩家: ", b.level, tableType)
		for _, t := range group {
			fmt.Printf("%d ", t.PlayerID)
		}
		fmt.Println()
	}
}

func (b *Bucket) getTableType() string {
	switch Level(b.level) {
	case LevelNewbie:
		return "新手桌"
	case LevelNormal:
		return "普通桌"
	case LevelHighWithdraw:
		return "回收桌"
	case LevelRisk:
		return "风控桌"
	default:
		return "未知桌"
	}
}

// ================= MinHeap =================

type MinHeap []*Task

func (h MinHeap) Len() int {
	return len(h)
}
func (h MinHeap) Less(i, j int) bool {
	if h[i].MatchAt.Equal(h[j].MatchAt) {
		return h[i].JoinAt.Before(h[j].JoinAt)
	}
	return h[i].MatchAt.Before(h[j].MatchAt)
}
func (h MinHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *MinHeap) Push(x interface{}) {
	n := len(*h)
	task := x.(*Task)
	task.index = n
	*h = append(*h, task)
}
func (h *MinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	task := old[n-1]
	old[n-1] = nil
	task.index = -1
	*h = old[:n-1]
	return task
}

// ================= Task =================

type Task struct {
	PlayerID int64
	Level    int
	JoinAt   time.Time // 玩家进入匹配系统的时间
	MatchAt  time.Time // 匹配触发时间（超时入桌）
	DeadAt   time.Time // 最迟业务处理时间（业务deadline）

	index     int
	canceled  bool
	cancelMux sync.RWMutex
}

func (t *Task) Cancel() {
	t.cancelMux.Lock()
	t.canceled = true
	t.cancelMux.Unlock()
}

func (t *Task) IsCanceled() bool {
	t.cancelMux.RLock()
	defer t.cancelMux.RUnlock()
	return t.canceled
}
