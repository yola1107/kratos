package match

import (
	"container/heap"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
)

// ================= MatchTask =================

type Level int

const (
	LevelNewbie       Level = iota // 新手
	LevelNormal                    // 普通/充值用户
	LevelHighWithdraw              // 提现比高
	LevelRisk                      // 风控/黑户/点控
)

type Task struct {
	PlayerID int64
	Level    int

	JoinAt  time.Time // 玩家进入匹配系统的时间
	MatchAt time.Time // 匹配触发时间（超时入桌）
	DeadAt  time.Time // 最迟业务处理时间（业务deadline）

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

// func (h MinHeap) Less(i, j int) bool {
// 	return h[i].MatchAt.Before(h[j].MatchAt)
// }

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

// ================= MatchBucket =================

type Bucket struct {
	level     int
	heap      MinHeap
	heapMu    sync.Mutex
	antsPool  *ants.Pool
	checkTick time.Duration
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// 创建匹配桶
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

// 定时检查是否有任务超时匹配
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
	// todo... 按权重分配人数
	deskSize := 4 // 桌子人数
	for i := 0; i < len(tasks); i += deskSize {
		end := i + deskSize
		if end > len(tasks) {
			end = len(tasks)
		}
		group := tasks[i:end]
		fmt.Printf("Level %d: 创建新桌，玩家: ", b.level)
		for _, t := range group {
			fmt.Printf("%d ", t.PlayerID)
		}
		fmt.Println()
		// 可扩展：投入桌子池系统（外部管理空桌列表）
	}
}

// ================= MatchSystem =================

type Pool struct {
	buckets   map[int]*Bucket
	bucketsMu sync.RWMutex
	antsPool  *ants.Pool
}

// NewPool 创建匹配系统
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

func (s *Pool) AddPlayer(playerID int64, level int) {
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
	deadAt := matchAt.Add(10 * time.Second) // 最长等 10 秒处理业务

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

func (s *Pool) CancelPlayer(task *Task) {
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
