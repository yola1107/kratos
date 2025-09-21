package table

import (
	"fmt"
	"sync"
	"time"

	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/game/ludo/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/conf"
)

/*

	StageID 游戏阶段ID
*/

type StageID int32

const (
	StWait     StageID = iota // 等待
	StReady                   // 准备
	StSendCard                // 发牌
	StDice                    // 投掷色子
	StMove                    // 移动棋子
	StResult                  // 结算
)

// StageTimeouts maps each stage to its timeout duration (in seconds).
var StageTimeouts = map[StageID]int64{
	StWait:     0,
	StReady:    0,
	StSendCard: 3,
	StDice:     7,
	StMove:     7,
	StResult:   3,
}

// StageNames maps each stage to its string name.
var StageNames = map[StageID]string{
	StWait:     "StWait",
	StReady:    "StReady",
	StSendCard: "StSendCard",
	StDice:     "StDice",
	StMove:     "StMove",
	StResult:   "StResult",
}

// String returns the string representation of the StageID.
func (s StageID) String() string {
	if name, ok := StageNames[s]; ok {
		return name
	}
	return fmt.Sprintf("StageID(%d)", s)
}

// Timeout returns the timeout duration of the stage.
func (s StageID) Timeout() int64 {
	if timeout, ok := StageTimeouts[s]; ok {
		if conf.IsFastMode() && (s == StDice || s == StMove) {
			return 5 // 5s
		}
		return timeout
	}
	log.Warnf("unknown stage: %d. use default timeout=0s", s)
	return 0
}

/*
Stage 游戏状态封装
*/

type Stage struct {
	mu       sync.RWMutex
	State    StageID
	Prev     StageID
	TimerID  int64
	StartAt  time.Time
	Duration time.Duration
}

func (s *Stage) Remaining() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	elapsed := time.Since(s.StartAt)
	if elapsed > s.Duration {
		return 0
	}
	return s.Duration - elapsed
}

func (s *Stage) GetState() StageID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

func (s *Stage) GetTimerID() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TimerID
}

func (s *Stage) Snap() (StageID, StageID, time.Duration, time.Time, int64) {
	s.mu.RLock()
	prev, state, dur, at, timerID := s.Prev, s.State, s.Duration, s.StartAt, s.TimerID
	s.mu.RUnlock()
	return prev, state, dur, at, timerID
}

func (s *Stage) Desc() string {
	prev, state, duration, _, _ := s.Snap()
	return fmt.Sprintf("[%v->%+v, %+v -> %v, dur=%v]",
		int32(prev), int32(state), prev, state, duration)
}

func (s *Stage) Set(state StageID, duration time.Duration, timerID int64) {
	s.mu.Lock() // 写锁
	defer s.mu.Unlock()
	s.Prev = s.State
	s.State = state
	s.StartAt = time.Now()
	s.Duration = duration
	s.TimerID = timerID
}

/*

	TYPE 桌子类型
*/

type TYPE int32

const (
	Normal TYPE = iota
)

func (t TYPE) String() string {
	switch t {
	case Normal:
		return "Normal"
	default:
		return "Unknown"
	}
}

// --------------------------------------

/*
	赢所有玩家（除自己外）底注之和（抽税20%），
	例：底注为100，3人玩，底注之和为：200，那么：
	如果赢：100 + 200 * （1-20%） = 260
	如果输：100（底注）
*/

// SettleObj 定义与实现
type SettleObj struct {
	Winner    *player.Player
	Users     []*player.Player
	BaseScore float64
	TaxRate   float64
	EndType   v1.FINISH_TYPE

	TaxFee   float64
	WinScore float64
	result   *v1.ResultPush
}

// GetResult 返回结算结果（只读）
func (s *SettleObj) GetResult() *v1.ResultPush {
	return s.result
}

// Settle 执行结算
func (s *SettleObj) Settle() error {
	if s.Winner == nil || len(s.Users) == 0 {
		return fmt.Errorf("invalid settle input: winner or users missing")
	}
	if s.BaseScore < 0 || s.TaxRate < 0 || s.TaxRate > 1 {
		return fmt.Errorf("invalid baseScore or taxRate")
	}

	winID := s.Winner.GetPlayerID()
	var (
		totalLost float64
		results   []*v1.PlayerResult
	)

	for _, p := range s.Users {
		if p == nil || p.GetPlayerID() == winID {
			continue
		}
		totalLost += s.BaseScore
		results = append(results, buildResult(p, false, -s.BaseScore))
	}

	tax := totalLost * s.TaxRate
	winScore := s.BaseScore + totalLost - tax
	s.WinScore = winScore
	s.TaxFee = tax

	results = append(results, buildResult(s.Winner, true, winScore))

	s.result = &v1.ResultPush{
		FinishType: s.EndType,
		WinnerID:   winID,
		Results:    results,
	}
	return nil
}

func buildResult(p *player.Player, isWinner bool, score float64) *v1.PlayerResult {
	return &v1.PlayerResult{
		UserID:   p.GetPlayerID(),
		ChairID:  p.GetChairID(),
		IsWinner: isWinner,
		WinScore: score,
		// HandCards:      p.GetCards(),
		// HandCardsScore: p.GetHandScore(),
	}
}
