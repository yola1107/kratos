package service

import (
	"context"

	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz"
)

// GetLoop 获取任务池
func (s *Service) GetLoop() work.ITaskLoop {
	return s.uc.GetLoop()
}

// OnSessionOpen 连接建立回调
func (s *Service) OnSessionOpen(sess *websocket.Session) {
	log.Infof("OnOpenFunc: %q", sess.ID())
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
	if _, err := s.uc.OnLoginReq(ctx, in); err != nil {
		return nil, err
	}
	return &v1.LoginRsp{}, nil
}

func (s *Service) OnLogoutReq(ctx context.Context, in *v1.LogoutReq) (*v1.LogoutRsp, error) {
	rs := s.uc.Swapper(ctx)
	if rs.Error != nil {
		return nil, rs.Error
	}

	rs.Table.OnExitGame(rs.Player, 0, "success")
	return &v1.LogoutRsp{}, nil
}

func (s *Service) OnReadyReq(ctx context.Context, in *v1.ReadyReq) (*v1.ReadyRsp, error) {
	rs := s.uc.Swapper(ctx)
	if rs.Error != nil {
		return nil, rs.Error
	}

	return &v1.ReadyRsp{}, nil
}

func (s *Service) OnSwitchTableReq(ctx context.Context, in *v1.SwitchTableReq) (*v1.SwitchTableRsp, error) {
	rs := s.uc.Swapper(ctx)
	if rs.Error != nil {
		return nil, rs.Error
	}

	s.uc.OnSwitchTableReq(rs)
	return &v1.SwitchTableRsp{}, nil
}

func (s *Service) OnSceneReq(ctx context.Context, in *v1.SceneReq) (*v1.SceneRsp, error) {
	rs := s.uc.Swapper(ctx)
	if rs.Error != nil {
		return nil, rs.Error
	}

	rs.Table.OnSceneReq(rs.Player, true)
	return &v1.SceneRsp{}, nil
}

func (s *Service) OnChatReq(ctx context.Context, in *v1.ChatReq) (*v1.ChatRsp, error) {
	rs := s.uc.Swapper(ctx)
	if rs.Error != nil {
		return nil, rs.Error
	}

	return &v1.ChatRsp{}, nil
}

func (s *Service) OnHostingReq(ctx context.Context, in *v1.HostingReq) (*v1.HostingRsp, error) {
	rs := s.uc.Swapper(ctx)
	if rs.Error != nil {
		return nil, rs.Error
	}

	return &v1.HostingRsp{}, nil
}

func (s *Service) OnForwardReq(ctx context.Context, in *v1.ForwardReq) (*v1.ForwardRsp, error) {
	rs := s.uc.Swapper(ctx)
	if rs.Error != nil {
		return nil, rs.Error
	}

	rs.Table.OnForwardReq(in.Type, in.Msg)
	return &v1.ForwardRsp{}, nil
}
