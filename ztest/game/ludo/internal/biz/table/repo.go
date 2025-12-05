package table

import (
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/conf"
)

// Repo 抽象接口
type Repo interface {
	GetLoop() work.Loop
	GetTimer() work.Scheduler
	GetRoomConfig() *conf.Room
	LogoutGame(p *player.Player, code int32, msg string)
}
