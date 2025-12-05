package main

import (
	"context"
	"fmt"
	"time"

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

// server 实现 GreeterGNETServer 接口
type server struct {
	gnetLoop work.Loop
}

func (s *server) GetLoop() work.Loop {
	return s.gnetLoop
}

func (s *server) SayHelloReq(ctx context.Context, req *v1.HelloRequest) (*v1.HelloReply, error) {
	log.Infof("[gnet] SayHelloReq received: %s", req.Name)
	return &v1.HelloReply{
		Message: fmt.Sprintf("Echo: %s", req.Name),
	}, nil
}

func (s *server) SayHello2Req(ctx context.Context, req *v1.Hello2Request) (*v1.Hello2Reply, error) {
	log.Infof("[gnet] SayHello2Req received: name=%s, seq=%d", req.Name, req.Seq)
	return &v1.Hello2Reply{
		Message: fmt.Sprintf("Echo2: name=%s, seq=%d", req.Name, req.Seq),
	}, nil
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

	// 注册 Greeter 服务（使用生成的协议）
	v1.RegisterGreeterGNETServer(gnetSrv, s)

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

/*

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

*/
