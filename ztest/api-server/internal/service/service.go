package service

import (
	"context"
	"errors"

	"github.com/google/wire"
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket"

	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gplayer"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gtable"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/iface"
)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(NewService)

var _ iface.IRoomRepo = (*Service)(nil)

var defaultPendingNum = 10000

// Service is a service.
type Service struct {
	v1.UnimplementedGreeterServer

	logger log.Logger
	uc     *biz.DataUsecase

	// room
	rc *conf.Room
	ws work.IWorkStore
	pm *gplayer.PlayerManager
	tm *gtable.TableManager
}

// NewService new a service.
func NewService(uc *biz.DataUsecase, logger log.Logger, c *conf.Room) (*Service, func(), error) {
	log.Infof("start server:\"%s\" version:%+v", conf.Name, conf.Version)
	log.Infof("GameID=%d ArenaID=%d ServerID=%s", conf.GameID, conf.ArenaID, conf.ServerID)

	ctx, cancel := context.WithCancel(context.Background())

	s := &Service{uc: uc, logger: logger}
	s.rc = c
	s.tm = gtable.NewTableManager(c, s)
	s.pm = gplayer.NewPlayerManager(c, s)
	s.ws = work.NewWorkStore(ctx, defaultPendingNum)

	cleanup := func() {
		log.NewHelper(logger).Info("closing the Room resources")
		cancel()
		s.pm.Close()
		s.tm.Close()
		s.ws.Stop()
	}
	return s, cleanup, errors.Join(s.ws.Start(), s.pm.Start(), s.tm.Start())
}

// GetLoop 获取任务池
func (s *Service) GetLoop() work.ITaskLoop {
	return s.ws
}

// GetTimer 获取定时器
func (s *Service) GetTimer() work.ITaskScheduler {
	return s.ws
}

// GetDataRepo 获取data
func (s *Service) GetDataRepo() biz.DataRepo {
	return s.uc.GetDataRepo()
}

// GetRoomConfig 获取房间配置
func (s *Service) GetRoomConfig() *conf.Room {
	return s.rc
}

// OnSessionOpen 连接建立回调
func (s *Service) OnSessionOpen(sess *websocket.Session) {
	log.Infof("OnOpenFunc: %q", sess.ID())
	// s.pm.CreatePlayer()
}

// OnSessionClose 连接关闭回调
func (s *Service) OnSessionClose(sess *websocket.Session) {
	log.Infof("OnCloseFunc: %q", sess.ID())
}

func (s *Service) OnTableEvent(tableID string, evt string) {
	log.Infof("Room handling table event:%+v %+v", tableID, evt)
}

func (s *Service) OnPlayerLeave(playerID string) {
	log.Infof("Room handling player leave:%+v", playerID)
}
