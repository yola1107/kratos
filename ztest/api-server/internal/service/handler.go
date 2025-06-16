package service

import (
	"context"

	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/model"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gplayer"
)

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
	p, err := s.pm.CreatePlayer(&gplayer.PlayerRaw{
		ID:      in.UserID,
		IP:      session.GetRemoteIP(),
		Session: session,
	})
	if p == nil {
		log.Warnf("loginRoom. UserID(%+v) err=%v", in.UserID, err)
		return nil, err
	}
	// 条件限制
	if err := s.tm.CanEnterRoom(p, in); err != nil {
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

func (s *Service) OnSwitchTableReq(ctx context.Context, in *v1.SwitchTableReq) (*v1.SwitchTableRsp, error) {
	rs := s.swapper(ctx)
	if rs.Error != nil {
		return nil, rs.Error
	}

	s.tm.OnSwitchTable(rs.Player)
	return &v1.SwitchTableRsp{}, nil
}

func (s *Service) OnSceneReq(ctx context.Context, in *v1.SceneReq) (*v1.SceneRsp, error) {
	rs := s.swapper(ctx)
	if rs.Error != nil {
		return nil, rs.Error
	}

	rs.Table.OnSceneReq(rs.Player, true)
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
