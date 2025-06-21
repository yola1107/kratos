package table

import (
	"fmt"
	"strings"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
)

// StageType 阶段ID枚举，模拟命名空间
var StageType = struct {
	Wait, Ready, SendCard, Action, SideShow, SideShowAni, WaitEnd, End int32
}{
	Wait:        0,
	Ready:       1,
	SendCard:    2,
	Action:      3,
	SideShow:    4,
	SideShowAni: 5,
	WaitEnd:     6,
	End:         7,
}

var StageNames = map[int32]string{
	StageType.Wait:        "等待",
	StageType.Ready:       "准备",
	StageType.SendCard:    "发牌",
	StageType.Action:      "操作",
	StageType.SideShow:    "提前比牌",
	StageType.SideShowAni: "比牌动画",
	StageType.WaitEnd:     "等待结束",
	StageType.End:         "游戏结束",
}

// StageTimeouts 每个阶段对应的超时时间（单位：秒）
var StageTimeouts = map[int32]int64{
	StageType.Ready:       2,  // 准备时间 (s)
	StageType.SendCard:    3,  // 发牌时间 (s)
	StageType.Action:      12, // 操作时间 (s)
	StageType.SideShow:    12, // 发起比牌 等待结束 (s)
	StageType.SideShowAni: 1,  // 同意提前比牌动画时间 (s)
	StageType.WaitEnd:     1,  // 等待结束时间 (s)
	StageType.End:         3,  // 结束等待下一个阶段时间 (s)
}

// descState 返回阶段描述
func descState(s int32) string {
	if name, ok := StageNames[s]; ok {
		return fmt.Sprintf("%s(%d)", name, s)
	}
	return fmt.Sprintf("未知(%d)", s)
}

// GetStageTimeout 返回阶段的超时时间
func GetStageTimeout(s int32) int64 {
	if timeout, ok := StageTimeouts[s]; ok {
		return timeout
	}
	log.Warnf("unknown stage: %d", s)
	return 0
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
//
// // ActionType 动作类型
// type ActionType int32
//
// const (
// 	AcCall      ActionType = 1 // 跟注
// 	AcRaise     ActionType = 2 // 加注
// 	AcSee       ActionType = 3 // 看牌
// 	AcPack      ActionType = 4 // 弃牌
// 	AcShow      ActionType = 5 // 比牌
// 	AcSide      ActionType = 6 // 提前比牌
// 	AcSideReply ActionType = 7 // 提前比牌回应
// )
//
// // actionNames 动作名称映射
// var actionNames = map[ActionType]string{
// 	AcCall:      "Call",
// 	AcRaise:     "Raise",
// 	AcSee:       "See",
// 	AcPack:      "Pack",
// 	AcShow:      "Show",
// 	AcSide:      "Side",
// 	AcSideReply: "SideReply",
// }
//
// // String 返回动作的字符串表示
// func (a ActionType) String() string {
// 	if s, ok := actionNames[a]; ok {
// 		return s
// 	}
// 	return "Unknown"
// }
//
// func descActions(actions ...ActionType) string {
// 	var names []string
// 	for _, a := range actions {
// 		names = append(names, a.String())
// 	}
// 	return strings.Join(names, " ")
// }

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
	var names []string
	for _, a := range actions {
		names = append(names, actionNames[a])
	}
	return strings.Join(names, " ")
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
