package gtable

import (
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/entity/gplayer"
)

type ITableEvent interface {
	GetLoop() work.ITaskLoop
	GetTimer() work.ITaskScheduler
	GetRoomConfig() *conf.Room
	LogoutGame(p *gplayer.Player, code int32, msg string)
	// OnPlayerLeave(playerID string)
}
