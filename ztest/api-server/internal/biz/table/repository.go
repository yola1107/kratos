package table

import (
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
)

// ITableRepo 抽象接口
type ITableRepo interface {
	GetLoop() work.ITaskLoop
	GetTimer() work.ITaskScheduler
	GetRoomConfig() *conf.Room
	LogoutGame(p *player.Player, code int32, msg string)
	// OnPlayerLeave(playerID string)
}
