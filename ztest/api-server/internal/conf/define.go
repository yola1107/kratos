package conf

import (
	"flag"
	"os"
)

const Name = "api-server"
const Version = "v0.0.1"
const GameID = 130

var ArenaID = 1   // 场ID: 1 2 3 4
var ServerID = "" // 房间ID

func init() {
	flag.IntVar(&ArenaID, "aid", 1, "specify the arena ID. base.StrToInt(os.Getenv(\"ARENAID\"))")
	flag.StringVar(&ServerID, "sid", os.Getenv("HOSTNAME"), "specify the server ID.")
}

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
	Normal TableType = iota
	Black
)

type TableType int32

func (t TableType) String() string {
	switch t {
	case Normal:
		return "Normal"
	case Black:
		return "Black"
	default:
		return "Unknown"
	}
}
