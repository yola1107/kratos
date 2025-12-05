package main

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/library/log/zap/conf"
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware/recovery"
	"github.com/yola1107/kratos/v2/transport/gnet"
	v1 "github.com/yola1107/kratos/v2/ztest/transport/api/helloworld/v1"
)

const (
	Name = "gnet-echo-server"
)

// 使用现有的 HelloRequest 和 HelloReply 作为 Echo 消息

type EchoRequest = v1.HelloRequest
type EchoReply = v1.HelloReply

// EchoServer gnet echo 服务接口
type EchoServer interface {
	GetLoop() work.Loop
	Echo(ctx context.Context, req *EchoRequest) (*EchoReply, error)
}

// server 实现 EchoServer 接口
type server struct {
	gnetLoop work.Loop
}

func (s *server) GetLoop() work.Loop {
	return s.gnetLoop
}

func (s *server) Echo(ctx context.Context, req *EchoRequest) (*EchoReply, error) {
	log.Infof("[gnet] Echo received: %s", req.Name)
	return &EchoReply{
		Message: fmt.Sprintf("Echo: %s", req.Name),
	}, nil
}

// echoHandler 处理 echo 请求的 handler
func echoHandler(srv interface{}, ctx context.Context, data []byte, interceptor gnet.UnaryServerInterceptor) ([]byte, error) {
	in := new(EchoRequest)
	if err := proto.Unmarshal(data, in); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to unmarshal request: %v", err)
	}

	doFunc := func(ctx context.Context, req *EchoRequest) ([]byte, error) {
		doRequest := func() ([]byte, error) {
			resp, err := srv.(EchoServer).Echo(ctx, req)
			if err != nil || resp == nil {
				return nil, err
			}
			return proto.Marshal(resp)
		}
		if loop := srv.(EchoServer).GetLoop(); loop != nil {
			return loop.PostAndWaitCtx(ctx, doRequest)
		}
		return doRequest()
	}

	if interceptor == nil {
		return doFunc(ctx, in)
	}

	info := &gnet.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/echo.EchoServer/Echo",
	}

	interceptorHandler := func(ctx context.Context, req interface{}) ([]byte, error) {
		r, ok := req.(*EchoRequest)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid Request Argument, expect: *EchoRequest, Not: %T", req)
		}
		return doFunc(ctx, r)
	}

	return interceptor(ctx, in, info, interceptorHandler)
}

// EchoServiceDesc echo 服务描述
var EchoServiceDesc = gnet.ServiceDesc{
	ServiceName: "echo.EchoServer",
	HandlerType: (*EchoServer)(nil),
	Methods: []gnet.MethodDesc{
		{
			Ops:        1001, // Echo 操作的 Ops
			MethodName: "Echo",
			Handler:    echoHandler,
		},
	},
}

func main() {
	logger := zap.NewLogger(conf.DefaultConfig(
		conf.WithAppName(Name),
	))
	defer logger.Close()
	log.SetLogger(logger)

	s := &server{}

	gnetSrv := gnet.NewServer(
		gnet.Address(":3200"),
		gnet.Timeout(5*time.Second),
		gnet.Middleware(
			recovery.Recovery(),
		),
	)

	// 注册 echo 服务
	gnetSrv.RegisterService(&EchoServiceDesc, s)

	app := kratos.New(
		kratos.Name(Name),
		kratos.Logger(logger),
		kratos.Server(gnetSrv),
		kratos.BeforeStart(func(ctx context.Context) error {
			s.gnetLoop = work.NewLoop(work.WithSize(1000))
			return s.gnetLoop.Start()
		}),
		kratos.AfterStop(func(ctx context.Context) error {
			if s.gnetLoop != nil {
				s.gnetLoop.Stop()
			}
			return nil
		}),
	)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
