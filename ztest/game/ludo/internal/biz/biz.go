package biz

import (
	"context"
	"errors"
	"time"

	"github.com/google/wire"
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/biz/robot"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/biz/table"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/conf"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(NewUsecase)

// 实现table.Repo接口
var _ table.Repo = (*Usecase)(nil)

// 实现robot.Repo接口
var _ robot.Repo = (*Usecase)(nil)

// 任务线程池容量
var defaultPendingNum = 10000

var defaultStatusInterval = 120 * time.Second

// DataRepo is a data repo.
type DataRepo interface {
	SavePlayer(ctx context.Context, p *player.BaseData) error
	LoadPlayer(ctx context.Context, playerID int64) (*player.BaseData, error)
	ExistPlayer(ctx context.Context, playerID int64) bool
}

// Usecase is a Data usecase.
type Usecase struct {
	repo DataRepo    // 数据访问层接口，持久化玩家信息
	log  *log.Helper // 日志记录器

	loop  work.ITaskLoop      // 任务循环
	timer work.ITaskScheduler // 定时任务

	rc *conf.Room      // 房间配置（从配置文件读取）
	pm *player.Manager // 玩家管理器
	tm *table.Manager  // 桌子管理器
	rm *robot.Manager  // 机器人管理器
}

// NewUsecase new a data usecase.
func NewUsecase(repo DataRepo, logger log.Logger, c *conf.Room) (*Usecase, func(), error) {
	ctx, cancel := context.WithCancel(context.Background())
	uc := &Usecase{repo: repo, log: log.NewHelper(logger), rc: c}

	// 初始化顺序：loop -> timer -> Table -> Player -> Robot
	uc.loop = work.NewAntsLoop(work.WithSize(defaultPendingNum))
	uc.timer = work.NewTaskScheduler(work.WithContext(ctx), work.WithExecutor(uc.loop), work.WithTick(time.Second/2))
	uc.tm = table.NewManager(c, uc)
	uc.pm = player.NewManager()
	uc.rm = robot.NewManager(c, uc)

	cleanup := func() {
		// log.Info("closing the Room resources")
		uc.tm.Close()
		uc.pm.Close()
		uc.rm.Stop()
		uc.timer.Stop()
		uc.loop.Stop()
		cancel() // 最后释放
	}
	return uc, cleanup, uc.start()
}

func (uc *Usecase) start() error {
	log.Infof("start server:%q version:%q", conf.Name, conf.Version)
	log.Infof("GameID=%d ArenaID=%d ServerID=%s", conf.GameID, conf.ArenaID, conf.ServerID)
	err := errors.Join(
		uc.loop.Start(),
		uc.tm.Start(),
		uc.pm.Start(),
		uc.rm.Start(),
	)
	uc.timer.Forever(defaultStatusInterval, uc.post)
	return err
}

func (uc *Usecase) post() {
	timers := uc.timer.Monitor()
	loops := uc.loop.Monitor()
	ps := uc.pm.Monitor()
	ai := uc.rm.Monitor()

	log.Infof("[monitor] [server] loop=%+v timer=%+v player=%+v AI=%+v", loops, timers, ps, ai)
}
