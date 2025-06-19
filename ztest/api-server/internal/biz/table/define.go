package table

const (
	StWait     = 0 // 等待
	StReady    = 1 // 准备
	StSendCard = 2 // 发牌
	StAction   = 3 // 操作
	// StShow     = 4  // 比牌
	StSideShow = 6  // 提前比牌
	StWaitSide = 5  // 等待提前比牌
	StWaitEnd  = 7  // 等待结束
	StEnd      = 10 // 游戏结束
)

const (
	StReadyTimeout    = 2  // 准备时间 (s)
	StSendCardTimeout = 3  // 发牌时间 (s)
	StActionTimeout   = 12 // 操作时间 (s)
	StShowTimeout     = 1  // 比牌动画时间 (s)
	StWaitEndTimeout  = 1  // 等待结束时间 (s)
	StEndTimeout      = 3  // 结束等待下一个阶段时间 (s)
)

var StageNames = map[int32]string{
	StWait:     "等待",
	StReady:    "准备",
	StSendCard: "发牌",
	StAction:   "操作",
	// StShow:     "比牌",
	StSideShow: "提前比牌",
	StWaitSide: "等待提前比牌",
	StWaitEnd:  "等待结束",
	StEnd:      "游戏结束",
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
	// case StShow:
	// 	return StShowTimeout
	case StSideShow:
		return StShowTimeout
	case StWaitSide:
		return StWaitEndTimeout
	case StWaitEnd:
		return StWaitEndTimeout
	case StEnd:
		return StEndTimeout
	}
	return 0
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
	PLAYER_CALL       = 1 // "跟注"
	PLAYER_RAISE      = 2 // "加注"
	PLAYER_SEE        = 3 // "看牌"
	PLAYER_PACK       = 4 // "弃牌"
	PLAYER_SHOW       = 5 // "比牌"
	PLAYER_SIDE       = 6 // "提前比牌"
	PLAYER_SIDE_REPLY = 7 // "提前比牌回应"
)
