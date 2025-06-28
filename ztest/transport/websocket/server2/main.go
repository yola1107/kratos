package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/library/log/zap/conf"
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware"
	"github.com/yola1107/kratos/v2/middleware/ratelimit"
	"github.com/yola1107/kratos/v2/middleware/recovery"
	"github.com/yola1107/kratos/v2/transport/grpc"
	"github.com/yola1107/kratos/v2/transport/http"
	"github.com/yola1107/kratos/v2/transport/websocket"
	v1 "github.com/yola1107/kratos/v2/ztest/transport/api/helloworld/v1"
	// "github.com/yola1107/kratos/contrib/log/zap/v2"
	// "github.com/yola1107/kratos/contrib/registry/etcd/v2"
	// etcdv3 "go.etcd.io/etcd/client/v3"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	Name   = "ws-server"
	wsLoop work.ITaskLoop
)

type server struct {
	v1.UnimplementedGreeterServer

	sessionsMap sync.Map
}

func (s *server) GetLoop() work.ITaskLoop {
	return wsLoop
}

// OnSessionOpen 连接建立回调
func (s *server) OnSessionOpen(session *websocket.Session) {
	// log.Infof("[ws] ConnectFunc callback. key=%q", session.ID())
	s.sessionsMap.Store(session.ID(), session)
}

// OnSessionClose 连接关闭回调
func (s *server) OnSessionClose(session *websocket.Session) {
	// log.Infof("[ws] DisConnectFunc callback. key=%q", session.ID())
	s.sessionsMap.Delete(session.ID())
}

func (s *server) SayHelloReq(ctx context.Context, in *v1.HelloRequest) (*v1.HelloReply, error) {
	return &v1.HelloReply{Message: in.Name}, nil
}

func (s *server) SayHello2Req(ctx context.Context, in *v1.Hello2Request) (*v1.Hello2Reply, error) {
	s.TestPushDataByID(ctx.Value("sessionID").(string)) // 测试push功能
	// panic("websocket server panic test")                      // 测试loop捕获panic
	// time.Sleep(6 * time.Second)                               // 测试context.DeadLine
	// return nil, nil                                           // 测试返回值为nil是否发送消息
	// return nil, fmt.Errorf("ws test handler return an error") // 测试返回值为err能否捕获到 error.code和error.msg
	// return nil, kerrors.New(201, "test", "test")// 测试返回值为err能否捕获到 error.code和error.msg
	return &v1.Hello2Reply{Message: fmt.Sprintf("ws server say hello. %s", in.Name)}, nil
}

func (s *server) TestPushDataByID(sessionID string) {
	session, err := s.GetSessionByID(sessionID)
	if err != nil {
		log.Errorf("TestPushDataByID err:%v", err)
		return
	}

	fn := func() {
		if err = session.Push(int32(v1.GameCommand_SayHello2Rsp), &v1.Hello2Reply{Message: "from server push."}); err != nil {
			log.Warnf("TestPushDataByID err:%v", err)
		}
	}

	if loop := s.GetLoop(); loop == nil {
		log.Warnf("loop is nil")
		fn()
	} else {
		loop.Post(fn)
	}
}

func (s *server) GetSessionByID(sessionID string) (session *websocket.Session, err error) {
	ss, ok := s.sessionsMap.Load(sessionID)
	if !ok {
		return nil, fmt.Errorf(" key=%q not exist", sessionID)
	}
	session = ss.(*websocket.Session)
	return
}

// GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o app main.go

func main() {
	// etcdClient, err := etcdv3.New(etcdv3.Config{
	// 	Endpoints: []string{"127.0.0.1:2379"},
	// })
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer etcdClient.Close()

	zapLogger := loadLogger()
	defer zapLogger.Close()

	s := &server{
		sessionsMap: sync.Map{},
	}

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

	wsSrv := websocket.NewServer(
		websocket.Address(":3102"),
		websocket.Timeout(time.Second*5),
		websocket.SentChanSize(64),

		websocket.Middleware(
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

	v1.RegisterGreeterServer(grpcSrv, s)
	v1.RegisterGreeterHTTPServer(httpSrv, s)
	v1.RegisterGreeterWebsocketServer(wsSrv, s)

	app := kratos.New(
		kratos.Name(Name),
		kratos.Server(
			httpSrv,
			grpcSrv,
			wsSrv,
		),
		kratos.Logger(zapLogger), // 使用自定义 Logger
		// kratos.Registrar(etcd.New(etcdClient)), // 注册中心 ETCD
		kratos.BeforeStart(func(ctx context.Context) error {
			wsLoop = work.NewAntsLoop(work.WithSize(10000))
			return wsLoop.Start()
		}),
		kratos.AfterStop(func(ctx context.Context) error {
			wsLoop.Stop()
			return nil
		}),
	)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func loadLogger() *zap.Logger {
	zapLogger := zap.NewLogger(conf.DefaultConfig(
		// conf.WithProduction(),
		conf.WithAppName(Name),
		conf.WithLevel("debug"),
		conf.WithDirectory("./logs"),
		conf.WithAlertEnable(true),
		conf.WithTelegramToken("7945687310:AAHA9tkUPV1ELEsVSLoDZe_Cc76wp7YdDVI"),
		conf.WithTelegramChatID("-4672893880"),
	))
	return zapLogger
}
