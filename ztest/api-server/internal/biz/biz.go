package biz

import (
	"context"
	"errors"

	"github.com/google/wire"
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/entity/gtable"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/entity/playermgr"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/entity/tablemgr"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(NewUsecase)

// 实现ITableEvent等接口
var _ gtable.ITableEvent = (*Usecase)(nil)

var defaultPendingNum = 10000

// Greeter is a Greeter model.
type Greeter struct {
	Hello string
}

// DataRepo is a data repo.
type DataRepo interface {
	Save(context.Context, *Greeter) (*Greeter, error)
	Update(context.Context, *Greeter) (*Greeter, error)
	FindByID(context.Context, int64) (*Greeter, error)
}

// Usecase is a Data usecase.
type Usecase struct {
	repo DataRepo
	log  *log.Helper

	// room
	rc *conf.Room
	ws work.IWorkStore
	pm *playermgr.PlayerManager
	tm *tablemgr.TableManager
}

// NewUsecase new a data usecase.
func NewUsecase(repo DataRepo, logger log.Logger, c *conf.Room) (*Usecase, func(), error) {
	uc := &Usecase{repo: repo, log: log.NewHelper(logger)}

	ctx, cancel := context.WithCancel(context.Background())
	uc.rc = c
	uc.tm = tablemgr.NewTableManager(c, uc)
	uc.pm = playermgr.NewPlayerManager()
	uc.ws = work.NewWorkStore(ctx, defaultPendingNum)

	cleanup := func() {
		log.NewHelper(logger).Info("closing the Room resources")
		cancel()
		// 	uc.pm.Close()
		// 	uc.tm.Close()
		uc.ws.Stop()
	}
	return uc, cleanup, errors.Join(uc.ws.Start())
}

// CreateGreeter creates a Greeter, and returns the new Greeter.
func (uc *Usecase) CreateGreeter(ctx context.Context, g *Greeter) (*Greeter, error) {
	uc.log.Infof("CreateGreeter: %v", g.Hello)
	return uc.repo.Save(ctx, g)
}

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
