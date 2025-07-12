package table

import (
	"fmt"

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
