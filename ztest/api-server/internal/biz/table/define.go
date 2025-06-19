package table

import (
	"github.com/yola1107/kratos/v2/log"
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
	StWaitEndTimeout     = 2  // 等待结束时间 (s)
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

func StageName(s int32) string {
	return StageNames[s]
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

const (
	Normal TYPE = iota
	Black
)

// TYPE 桌子类型
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
	AcCall      = int32(1) // "跟注"
	AcRaise     = int32(2) // "加注"
	AcSee       = int32(3) // "看牌"
	AcPack      = int32(4) // "弃牌"
	AcShow      = int32(5) // "比牌"
	AcSide      = int32(6) // "提前比牌"
	AcSideReply = int32(7) // "提前比牌回应"
)
