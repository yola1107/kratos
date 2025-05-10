package main

import (
	"context"
	"sync"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware/recovery"
	"github.com/yola1107/kratos/v2/transport/grpc"
	"github.com/yola1107/kratos/v2/transport/http"
	"github.com/yola1107/kratos/v2/transport/websocket"
	v2 "github.com/yola1107/kratos/v2/ztest/transport/api/helloworld/v1"
	//"github.com/yola1107/kratos/contrib/log/zap/v2"
	//"github.com/yola1107/kratos/contrib/registry/etcd/v2"
	//etcdv3 "go.etcd.io/etcd/client/v3"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	Name = "ws-server"
)

type server struct {
	v2.UnimplementedGreeterServer

	sessionsMap sync.Map
}

func (s *server) IsLoopFunc(f string) bool {
	return false
}
func (s *server) SayHelloReq(ctx context.Context, in *v2.HelloRequest) (*v2.HelloReply, error) {
	return &v2.HelloReply{Message: in.Name}, nil
}

func (s *server) SayHello2Req(ctx context.Context, in *v2.Hello2Request) (*v2.Hello2Reply, error) {
	session := ctx.Value("session")
	if session != nil {
		v2.GetLoop().Post(func() {
			ss := session.(*websocket.Session)
			err := ss.Push(2, &v2.Hello2Reply{Message: "server push."})
			if err != nil {
				log.Infof("push err:%v", err)
			}
		})
	}
	return &v2.Hello2Reply{}, nil
}

// OnOpenFunc 连接建立回调
func (s *server) OnOpenFunc(session *websocket.Session) {
	s.sessionsMap.Store(session.ID(), session)
}

// OnCloseFunc 连接关闭回调
func (s *server) OnCloseFunc(session *websocket.Session) {
	s.sessionsMap.Delete(session.ID())
}

// GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o app main.go

func main() {
	//etcdClient, err := etcdv3.New(etcdv3.Config{
	//	Endpoints: []string{"127.0.0.1:2379"},
	//})
	//if err != nil {
	//	log.Fatal(err)
	//}
	//defer etcdClient.Close()

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
		websocket.Middleware(
			recovery.Recovery(),
		),
		websocket.OnOpenFunc(s.OnOpenFunc),
		websocket.OnCloseFunc(s.OnCloseFunc),
	)

	v2.RegisterGreeterServer(grpcSrv, s)
	v2.RegisterGreeterHTTPServer(httpSrv, s)
	v2.RegisterGreeterWebsocketServer(wsSrv, s)

	app := kratos.New(
		kratos.Name(Name),
		kratos.Server(
			httpSrv,
			grpcSrv,
			wsSrv,
		),
		kratos.Logger(zapLogger), // 使用自定义 Logger
		//kratos.Registrar(etcd.New(etcdClient)), // 注册中心 ETCD
	)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func loadLogger() *zap.Logger {
	c := zap.DefaultConfig(
		zap.WithMode(zap.Production),
		zap.WithDirectory("./logs"),
		zap.WithFilename(Name+".log"),
		zap.WithErrorFilename(Name+"_error.log"),
		zap.WithPrefix(Name),
		//zap.WithToken(os.Getenv("TG_TOKEN")),
		//zap.WithChatID(os.Getenv("TG_CHAT_ID")),
		zap.WithToken("7945687310:AAHA9tkUPV1ELEsVSLoDZe_Cc76wp7YdDVI"),
		zap.WithChatID("-4672893880"),
		//zap.WithMaxBatchCnt(1),
		//zap.WithRateLimiter(time.Second*5),
		//zap.WithThreshold(zapcore.WarnLevel),
	)
	zapLogger := zap.New(c)
	return zapLogger
}
