package main

import (
	"context"
	"fmt"

	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/library/log/zap/conf"
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/metadata"
	"github.com/yola1107/kratos/v2/middleware"
	"github.com/yola1107/kratos/v2/middleware/ratelimit"
	v2 "github.com/yola1107/kratos/v2/ztest/transport/api/helloworld/v1"
	gproto "google.golang.org/protobuf/proto"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware/recovery"
	"github.com/yola1107/kratos/v2/transport/grpc"
	"github.com/yola1107/kratos/v2/transport/http"
	"github.com/yola1107/kratos/v2/transport/tcp"
	// "google.golang.org/grpc/metadata"
)

// go build -ldflags "-X main.Version=x.y.z"

var (
	Name = "helloworld"
)

type server struct {
	v2.UnimplementedGreeterServer

	tcpLoop work.Loop
	session *tcp.ChanList
}

var seed int64

func (s *server) SetCometChan(cl *tcp.ChanList, cs *tcp.Server) {
	s.session = cl
}

func (s *server) GetTCPLoop() work.Loop {
	return s.tcpLoop
}

func (s *server) SayHelloReq(ctx context.Context, in *v2.HelloRequest) (*v2.HelloReply, error) {
	// panic("tcp panic test")
	return &v2.HelloReply{Message: "SayHelloReq. Hello " + in.Name}, nil
}

func (s *server) SayHello2Req(ctx context.Context, in *v2.Hello2Request) (*v2.Hello2Reply, error) {
	s.testPush(ctx)
	// panic("tcp panic test")
	// return nil, fmt.Errorf("666. tcp test err")
	return &v2.Hello2Reply{Message: "tcp server say hello. " + in.Name}, nil
}

func (s *server) testPush(ctx context.Context) {
	seed++

	// 获取玩家的sessionID
	mid := ""
	// 从 ctx 中提取 metadata
	md, ok := metadata.FromServerContext(ctx)
	if !ok {
		fmt.Println("无法获取 metadata")
	} else {
		// 获取 mid
		mid = md.Get("mid")
	}
	for i := 0; i < 1; i++ {
		bytes, _ := gproto.Marshal(&v2.Hello2Reply{Message: fmt.Sprintf("Reply_%d", seed)})
		s.session.PushChan <- &tcp.PushData{
			Mid:  mid,
			Ops:  int32(v2.GameCommand_SayHello2Rsp),
			Data: bytes,
		}
	}
}

func main() {
	logger := zap.NewLogger(conf.DefaultConfig(
		conf.WithAppName(Name),
	))

	s := &server{}
	httpSrv := http.NewServer(
		http.Address(":8000"),
		http.Middleware(
			recovery.Recovery(),
		),
	)
	grpcSrv := grpc.NewServer(
		grpc.Address(":9000"),
		grpc.Middleware(
			recovery.Recovery(),
		),
	)
	tcpSrv := tcp.NewServer(
		tcp.Address(":3101"),
		tcp.Middleware(
			recovery.Recovery(),
			// logging.Server(zapLogger),
			ratelimit.Server(),
			func(handler middleware.Handler) middleware.Handler {
				return func(ctx context.Context, req any) (any, error) {
					// log.Info("<M1>请求开始:")
					// defer log.Info("<M1>请求结束")
					return handler(ctx, req)
				}
			},
			func(handler middleware.Handler) middleware.Handler {
				return func(ctx context.Context, req any) (any, error) {
					// log.Info("<logging>请求开始:")
					// defer log.Info("<logging>请求结束:")
					return handler(ctx, req)
				}
			},
		),
	)

	v2.RegisterGreeterServer(grpcSrv, s)
	v2.RegisterGreeterHTTPServer(httpSrv, s)
	v2.RegisterGreeterTCPServer(tcpSrv, s)
	app := kratos.New(
		kratos.Name(Name),
		kratos.Logger(logger),
		kratos.Server(
			httpSrv,
			grpcSrv,
			tcpSrv,
		),
		kratos.BeforeStart(func(ctx context.Context) error {
			s.tcpLoop = work.NewLoop(work.WithSize(10000))
			return s.tcpLoop.Start()
		}),
		kratos.AfterStop(func(ctx context.Context) error {
			s.tcpLoop.Stop()
			return nil
		}),
	)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
