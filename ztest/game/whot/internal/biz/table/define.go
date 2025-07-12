package table

import (
	"fmt"
	"sync"
	"time"

	"github.com/yola1107/kratos/v2/log"
)

/*

	StageID 游戏阶段ID
*/

type StageID int32

const (
	StWait     StageID = iota // 等待
	StReady                   // 准备
	StSendCard                // 发牌
	StPlaying                 // 操作
	StWaitEnd                 // 等待结束
	StEnd                     // 游戏结束
)

// StageTimeouts maps each stage to its timeout duration (in seconds).
var StageTimeouts = map[StageID]int64{
	StWait:     0,
	StReady:    0,
	StSendCard: 3,
	StPlaying:  8,
	StWaitEnd:  1,
	StEnd:      5,
}

// StageNames maps each stage to its string name.
var StageNames = map[StageID]string{
	StWait:     "StWait",
	StReady:    "StReady",
	StSendCard: "StSendCard",
	StPlaying:  "StPlaying",
	StWaitEnd:  "StWaitEnd",
	StEnd:      "StEnd",
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
