package table

import (
	"fmt"
	"strings"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
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
	StReady:       2,
	StSendCard:    3,
	StAction:      12,
	StSideShow:    12,
	StSideShowAni: 1,
	StWaitEnd:     1,
	StEnd:         3,
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
	log.Warnf("unknown stage: %d", s)
	return 0
}

/*
	TYPE 桌子类型
*/

// TYPE represents the table type.
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

// CompareType defines the type of comparison during the game.
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
