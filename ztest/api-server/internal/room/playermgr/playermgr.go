package playermgr

import (
	"sync"
)

var (
	playerMap sync.Map // key: playerID, value: *Player
)

func Init() {
	// 启动相关定时、回收、广播逻辑
}
