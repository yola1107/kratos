package biz

import (
	"context"
	"errors"

	"github.com/google/wire"
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/robot"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/table"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(NewUsecase)

// 实现table.Repo接口
var _ table.Repo = (*Usecase)(nil)

var defaultPendingNum = 10000

// DataRepo is a data repo.
type DataRepo interface {
	SavePlayer(ctx context.Context, p *player.BaseData) error
	LoadPlayer(ctx context.Context, playerID int64) (*player.BaseData, error)
}

// Usecase is a Data usecase.
type Usecase struct {
	repo DataRepo
	log  *log.Helper

	// room
	rc *conf.Room
	ws work.IWorkStore
	pm *player.Manager
	tm *table.Manager
	rm *robot.Manager
}

// NewUsecase new a data usecase.
func NewUsecase(repo DataRepo, logger log.Logger, c *conf.Room) (*Usecase, func(), error) {
	log.Infof("start server:\"%s\" version:%+v", conf.Name, conf.Version)
	log.Infof("GameID=%d ArenaID=%d ServerID=%s", conf.GameID, conf.ArenaID, conf.ServerID)

	uc := &Usecase{repo: repo, log: log.NewHelper(logger)}

	ctx, cancel := context.WithCancel(context.Background())
	uc.rc = c
	uc.ws = work.NewWorkStore(ctx, defaultPendingNum)
	uc.tm = table.NewManager(c, uc)
	uc.pm = player.NewManager()
	uc.rm = robot.NewManager(c, uc)

	cleanup := func() {
		log.Info("closing the Room resources")
		cancel()
		// 	uc.pm.Close()
		// 	uc.tm.Close()
		uc.ws.Stop()
		uc.rm.Stop()
	}
	return uc, cleanup, errors.Join(uc.ws.Start(), uc.rm.Start())
}

// GetLoop 获取任务队列
func (uc *Usecase) GetLoop() work.ITaskLoop {
	return uc.ws
}

// GetTimer 获取定时器
func (uc *Usecase) GetTimer() work.ITaskScheduler {
	return uc.ws
}

// GetRoomConfig 获取房间配置
func (uc *Usecase) GetRoomConfig() *conf.Room {
	return uc.rc
}

// GetTableList 获取桌子列表
func (uc *Usecase) GetTableList() []*table.Table {
	return uc.tm.GetTableList()
}
