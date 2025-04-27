package service

import (
	"context"

	"github.com/yola1107/kratos/v2/errors"
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/model"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gplayer"
)

func (s *Service) GetDataRepo() biz.DataRepo {
	return s.uc.GetDataRepo()
}

func (s *Service) OnTableEvent(tableID string, evt string) {
	log.Infof("Room handling table event:%+v %+v", tableID, evt)
}

func (s *Service) GetRoomConfig() *conf.Room {
	return s.rc
}

func (s *Service) OnPlayerLeave(playerID string) {
	log.Infof("Room handling player leave:%+v", playerID)
}

// GetTimer 获取定时器
func (s *Service) GetTimer() work.ITaskScheduler {
	return s.ws
}

// GetLoop 获取任务池
func (s *Service) GetLoop() work.ITaskLoop {
	return s.ws
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

// SayHelloReq implements helloworld.GreeterServer.
func (s *Service) SayHelloReq(ctx context.Context, in *v1.HelloRequest) (*v1.HelloReply, error) {
	g, err := s.uc.CreateGreeter(ctx, &biz.Greeter{Hello: in.Name})
	if err != nil {
		return nil, err
	}
	return &v1.HelloReply{Message: "Hello " + g.Hello}, nil
}

func (s *Service) OnLoginReq(ctx context.Context, in *v1.LoginReq) (*v1.LoginRsp, error) {
	if s.pm.ExistPlayer(in.UserID) {
		return s.reconnect(ctx, in)
	}
	return s.loginRoom(ctx, in)
}

func (s *Service) reconnect(ctx context.Context, in *v1.LoginReq) (*v1.LoginRsp, error) {
	session := s.getSession(ctx)
	if session == nil {
		return nil, model.ErrSessionNotFound
	}
	s.ws.Post(func() {
		p := s.pm.GetPlayerByID(in.UserID)
		if p == nil {
			return
		}
		t := s.tm.GetTable(p.GetTableID())
		if t == nil {
			return
		}
		p.UpdateSession(session)
		t.ReEnter(p)
	})
	return &v1.LoginRsp{}, nil
}

func (s *Service) loginRoom(ctx context.Context, in *v1.LoginReq) (*v1.LoginRsp, error) {
	session := s.getSession(ctx)
	if session == nil {
		return nil, model.ErrSessionNotFound
	}
	p := s.pm.CreatePlayer(&gplayer.PlayerRaw{
		ID:      in.UserID,
		IP:      session.GetRemoteIP(),
		Session: session,
	})
	if p == nil {
		log.Warnf("loginRoom. UserID(%+v) err=%v", in.UserID, model.ErrPlayerNotFound)
		return nil, model.ErrPlayerNotFound
	}
	// 条件限制
	if err := s.canEnterRoom(p, in); err != nil {
		log.Warnf("loginRoom. UserID(%+v) err=%v", in.UserID, err)
		s.pm.ExitGame(p, err.Code, err.Message) // 释放玩家
		return nil, err
	}
	s.ws.Post(func() {
		if !s.tm.ThrowInto(p) {
			log.Errorf("ThrowInto failed. pid:%d", in.UserID)
			s.pm.ExitGame(p, 0, "throw into table failed")
			return
		}
	})
	return &v1.LoginRsp{}, nil
}

// CanEnterRoom 检查是否能进房
func (s *Service) canEnterRoom(p *gplayer.Player, in *v1.LoginReq) (err *errors.Error) {
	if p == nil {
		return model.ErrPlayerNotFound
	}
	// 校验token
	if in.Token == "" {
		return model.ErrTokenFail
	}

	// room limit
	m := p.GetMoney()
	c := s.rc.Game
	if m < c.MinMoney {
		return model.ErrMoneyBelowMinLimit
	}
	if m > c.MaxMoney && c.MaxMoney != -1 {
		return model.ErrMoneyOverMaxLimit
	}
	if m < c.BaseMoney {
		return model.ErrMoneyBelowBaseLimit
	}
	if m < c.BaseMoney {
		return model.ErrVipLimit
	}
	return nil
}

func (s *Service) OnLogoutReq(ctx context.Context, in *v1.LogoutReq) (*v1.LogoutRsp, error) {
	rs := s.swapper(ctx)
	if rs.Error != nil {
		return nil, rs.Error
	}

	rs.Table.OnExitGame(rs.Player, 0, "success")
	return &v1.LogoutRsp{}, nil
}

func (s *Service) OnReadyReq(ctx context.Context, in *v1.ReadyReq) (*v1.ReadyRsp, error) {
	rs := s.swapper(ctx)
	if rs.Error != nil {
		return nil, rs.Error
	}

	return &v1.ReadyRsp{}, nil
}

func (s *Service) OnSwitchChairReq(ctx context.Context, in *v1.SwitchChairReq) (*v1.SwitchChairRsp, error) {
	rs := s.swapper(ctx)
	if rs.Error != nil {
		return nil, rs.Error
	}

	return &v1.SwitchChairRsp{}, nil
}

func (s *Service) OnSceneReq(ctx context.Context, in *v1.SceneReq) (*v1.SceneRsp, error) {
	rs := s.swapper(ctx)
	if rs.Error != nil {
		return nil, rs.Error
	}

	return &v1.SceneRsp{}, nil
}

func (s *Service) OnChatReq(ctx context.Context, in *v1.ChatReq) (*v1.ChatRsp, error) {
	rs := s.swapper(ctx)
	if rs.Error != nil {
		return nil, rs.Error
	}

	return &v1.ChatRsp{}, nil
}

func (s *Service) OnHostingReq(ctx context.Context, in *v1.HostingReq) (*v1.HostingRsp, error) {
	rs := s.swapper(ctx)
	if rs.Error != nil {
		return nil, rs.Error
	}

	return &v1.HostingRsp{}, nil
}

func (s *Service) OnForwardReq(ctx context.Context, in *v1.ForwardReq) (*v1.ForwardRsp, error) {
	rs := s.swapper(ctx)
	if rs.Error != nil {
		return nil, rs.Error
	}

	rs.Table.OnForwardReq(in.Type, in.Msg)
	return &v1.ForwardRsp{}, nil
}
