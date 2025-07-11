package table

import (
	"fmt"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/calc"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/player"
)

/*

	StageID 游戏阶段ID
*/

type StageID int32

const (
	StWait        StageID = iota // 等待
	StReady                      // 准备
	StSendCard                   // 发牌
	StAction                     // 操作
	StSideShow                   // 发起提前比牌，等待应答
	StSideShowAni                // 同意提前比牌动画
	StWaitEnd                    // 等待结束
	StEnd                        // 游戏结束
)

// StageTimeouts maps each stage to its timeout duration (in seconds).
var StageTimeouts = map[StageID]int64{
	StWait:        0,
	StReady:       1,
	StSendCard:    3,
	StAction:      12,
	StSideShow:    12,
	StSideShowAni: 1,
	StWaitEnd:     1,
	StEnd:         5,
}

// StageNames maps each stage to its string name.
var StageNames = map[StageID]string{
	StWait:        "StWait",
	StReady:       "StReady",
	StSendCard:    "StSendCard",
	StAction:      "StAction",
	StSideShow:    "StSideShow",
	StSideShowAni: "StSideShowAni",
	StWaitEnd:     "StWaitEnd",
	StEnd:         "StEnd",
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
	Black
)

func (t TYPE) String() string {
	switch t {
	case Normal:
		return "Normal"
	case Black:
		return "Black"
	default:
		return "Unknown"
	}
}

/*

	CompareType 比牌类型
*/

type CompareType int32

const (
	CompareShow     CompareType = iota + 1 // 普通比牌
	CompareAllShow                         // 全部比牌
	CompareSideShow                        // 提前比牌
)

var compareNames = map[CompareType]string{
	CompareShow:     "Show",
	CompareAllShow:  "AllShow",
	CompareSideShow: "SideShow",
}

func (t CompareType) String() string {
	if s, ok := compareNames[t]; ok {
		return s
	}
	return fmt.Sprintf("CompareType(%d)", t)
}

/*

	ActionRet 检查动作结果及动作错误码
*/

const (
	ErrOK int32 = iota
	ErrInvalidStage
	ErrNotEnoughMoney
	ErrorAlreadySeen
	ErrSideNotSeen
	ErrTargetInvalid
)

type ActionRet struct { // 检查结果
	Code    int32
	Money   float64
	Target  *player.Player
	Message string // 可选，用于调试或客户端提示
}

/*

	LogoutInfo 离开玩家的牌局数据
*/

type LogoutInfo struct {
	SID        int32
	UID        int64
	Money      float64
	StartMoney float64 // 局数开始时的金币
	Bet        float64
	Card       calc.HandCard
	PlayCount  int32 // 玩的回合数
	SeeRound   int32 // 看牌回合数
}

func newLogoutInfo(p *player.Player) LogoutInfo {
	return LogoutInfo{
		SID:        p.GetChairID(),
		UID:        p.GetPlayerID(),
		Money:      p.GetAllMoney(),
		StartMoney: 0,
		Bet:        p.GetBet(),
		Card:       p.GetHand(),
		PlayCount:  p.GetPlayCnt(),
		SeeRound:   0,
	}
}
