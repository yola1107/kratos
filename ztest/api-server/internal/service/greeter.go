package service

import (
	"context"

	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/room"
)

// GreeterService is a greeter service.
type GreeterService struct {
	v1.UnimplementedGreeterServer

	uc *biz.GreeterUsecase
	rc *room.Room
}

// NewGreeterService new a greeter service.
func NewGreeterService(uc *biz.GreeterUsecase, rc *room.Room, logger log.Logger) *GreeterService {
	return &GreeterService{uc: uc, rc: rc}
}

// SayHelloReq implements helloworld.GreeterServer.
func (s *GreeterService) SayHelloReq(ctx context.Context, in *v1.HelloRequest) (*v1.HelloReply, error) {
	g, err := s.uc.CreateGreeter(ctx, &biz.Greeter{Hello: in.Name})
	if err != nil {
		return nil, err
	}
	return &v1.HelloReply{Message: "Hello " + g.Hello}, nil
}

// SayHello2Req implements helloworld.GreeterServer.
func (s *GreeterService) SayHello2Req(ctx context.Context, in *v1.Hello2Request) (*v1.Hello2Reply, error) {
	g, err := s.uc.CreateGreeter(ctx, &biz.Greeter{Hello: in.Name})
	if err != nil {
		return nil, err
	}
	return &v1.Hello2Reply{Message: "Hello " + g.Hello}, nil
}

//
//func (s *GreeterService) SetCometChan(cl *tcp.ChanList, cs *tcp.Server) {}
//
//func (s *GreeterService) IsLoopFunc(f string) (isLoop bool) { return false }

func (s *GreeterService) GetLoop() work.ITaskLoop {
	return nil
}

// OnOpenFunc 连接建立回调
func (s *GreeterService) OnOpenFunc(sess *websocket.Session) {
	log.Infof("OnOpenFunc: %+v", sess.ID())
}

// OnCloseFunc 连接关闭回调
func (s *GreeterService) OnCloseFunc(sess *websocket.Session) {
	log.Infof("OnCloseFunc: %+v", sess.ID())
}
