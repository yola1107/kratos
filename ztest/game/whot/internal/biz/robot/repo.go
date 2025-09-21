package robot

import (
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/table"
)

// Repo 抽象接口
type Repo interface {
	GetTimer() work.Scheduler
	CreateRobot(raw *player.Raw) (*player.Player, error)
	GetTableList() []*table.Table
}
