package table

import (
	"fmt"
	"strings"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
)

const (
	StWait        = iota // 等待
	StReady              // 准备
	StSendCard           // 发牌
	StAction             // 操作
	StSideShow           // 发起提前比牌 等待应答
	StSideShowAni        // 同意提前比牌动画 (sideshow)
	StWaitEnd            // 等待结束
	StEnd                // 游戏结束
)

const (
	StReadyTimeout       = 2  // 准备时间 (s)
	StSendCardTimeout    = 3  // 发牌时间 (s)
	StActionTimeout      = 12 // 操作时间 (s)
	StSideShowTimeout    = 12 // 发起比牌 等待结束 (s)
	StSideShowAniTimeout = 1  // 同意提前比牌动画时间 (s)
	StWaitEndTimeout     = 1  // 等待结束时间 (s)
	StEndTimeout         = 3  // 结束等待下一个阶段时间 (s)
)

var StageNames = map[int32]string{
	StWait:        "等待",
	StReady:       "准备",
	StSendCard:    "发牌",
	StAction:      "操作",
	StSideShow:    "提前比牌",
	StSideShowAni: "比牌动画",
	StWaitEnd:     "等待结束",
	StEnd:         "游戏结束",
}

func descState(s int32) string {
	return fmt.Sprintf("%s(%d)", StageNames[s], s)
}

func GetStageTimeout(s int32) int64 {
	switch s {
	case StReady:
		return StReadyTimeout
	case StSendCard:
		return StSendCardTimeout
	case StAction:
		return StActionTimeout
	case StSideShow:
		return StSideShowTimeout
	case StSideShowAni:
		return StSideShowAniTimeout
	case StWaitEnd:
		return StWaitEndTimeout
	case StEnd:
		return StEndTimeout
	default:
		log.Warnf("unknow stage name:%d", s)
		return 0
	}
}

/*
	TYPE 桌子类型
*/

type TYPE int32

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

const (
	Normal TYPE = iota
	Black
)

/*
	动作类型
*/

const (
	AcCall      = int32(1) // "跟注"
	AcRaise     = int32(2) // "加注"
	AcSee       = int32(3) // "看牌"
	AcPack      = int32(4) // "弃牌"
	AcShow      = int32(5) // "比牌"
	AcSide      = int32(6) // "提前比牌"
	AcSideReply = int32(7) // "提前比牌回应"
)

var actionNames = map[int32]string{
	AcCall:      "Call",
	AcRaise:     "Raise",
	AcSee:       "See",
	AcPack:      "Pack",
	AcShow:      "Show",
	AcSide:      "Side",
	AcSideReply: "SideReply",
}

func descActions(actions ...int32) string {
	sb := strings.Builder{}
	for _, a := range actions {
		sb.WriteString(actionNames[a] + " ")
	}
	return sb.String()
}

/*
	CompareType 比牌类型
*/

type CompareType int32

const (
	CompareShow CompareType = iota + 1
	CompareAllShow
	CompareSideShow
)

func (t CompareType) String() string {
	switch t {
	case CompareShow:
		return "CompareShow"
	case CompareAllShow:
		return "CompareAllShow"
	case CompareSideShow:
		return "CompareSideShow"
	default:
		return "Unknown"
	}
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
