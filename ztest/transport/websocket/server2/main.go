package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware/recovery"
	"github.com/yola1107/kratos/v2/transport/grpc"
	"github.com/yola1107/kratos/v2/transport/http"
	"github.com/yola1107/kratos/v2/transport/websocket"
	v1 "github.com/yola1107/kratos/v2/ztest/transport/api/helloworld/v1"
	//"github.com/yola1107/kratos/contrib/log/zap/v2"
	//"github.com/yola1107/kratos/contrib/registry/etcd/v2"
	//etcdv3 "go.etcd.io/etcd/client/v3"
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

func (s *server) SayHelloReq(ctx context.Context, in *v1.HelloRequest) (*v1.HelloReply, error) {
	return &v1.HelloReply{Message: in.Name}, nil
}

func (s *server) SayHello2Req(ctx context.Context, in *v1.Hello2Request) (*v1.Hello2Reply, error) {
	//session := ctx.Value("session")
	//if session != nil {
	//
	//	s.GetLoop().Post(func() {
	//		ss := session.(*websocket.Session)
	//		//// push未定义的 cmd
	//		//if err := ss.Push(9876, &v1.Hello2Reply{Message: "from server push."}); err != nil {
	//		//	log.Infof("push err:%v", err)
	//		//}
	//		//
	//		////
	//		//resp := &v1.Hello2Reply{Message: fmt.Sprintf("from server push. %s", in.Name)}
	//		//if err := ss.Push(int32(v1.GameCommand_SayHello2Rsp), resp); err != nil {
	//		//	log.Infof("push err:%v", err)
	//		//}
	//
	//		log.Infof("sessionId:%v", ss.ID())
	//	})
	//}

	if session, err := s.GetSessionByID(ctx.Value("sessionID").(string)); err == nil {
		if err = session.Push(9876, &v1.Hello2Reply{Message: "from server push."}); err != nil {
			log.Infof("push err:%v", err)
		}
	} else {
		log.Errorf("GetSessionByID err:%v", err)
	}
	return &v1.Hello2Reply{Message: fmt.Sprintf("ws server say hello. %s", in.Name)}, nil
}

// OnOpenFunc 连接建立回调
func (s *server) OnOpenFunc(session *websocket.Session) {
	log.Infof("[ws] OnOpenFunc session:%+v", session.ID())
	s.sessionsMap.Store(session.ID(), session)
}

// OnCloseFunc 连接关闭回调
func (s *server) OnCloseFunc(session *websocket.Session) {
	log.Infof("[ws] OnCloseFunc session:%+v", session.ID())
	s.sessionsMap.Delete(session.ID())
}

func (s *server) GetSessionByID(sessionID string) (session *websocket.Session, err error) {
	ss, ok := s.sessionsMap.Load(sessionID)
	if !ok {
		return nil, fmt.Errorf("session:%s not exist", sessionID)
	}
	session = ss.(*websocket.Session)
	return
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
		//kratos.Registrar(etcd.New(etcdClient)), // 注册中心 ETCD
		kratos.BeforeStart(func(ctx context.Context) error {
			wsLoop = work.NewAntsLoop(10000)
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
	zapLogger, err := zap.NewLogger(
		//zap.WithProduction(),
		zap.WithLevel("debug"),
		zap.WithDirectory("./logs"),
		zap.WithFilename(Name+".log"),
		zap.WithErrorFilename(Name+"_error.log"),
		zap.WithMaxSizeMB(10),  //10M
		zap.WithMaxAgeDays(10), //1天
		zap.WithMaxBackups(10),
		zap.WithCompress(true),
		zap.WithLocalTime(true),
		zap.WithSensitiveKeys([]string{"pwd", "password"}),
		zap.WithPrefix(Name),
		//zap.WithToken(os.Getenv("TG_TOKEN")),
		//zap.WithChatID(os.Getenv("TG_CHAT_ID")),
		zap.WithToken("7945687310:AAHA9tkUPV1ELEsVSLoDZe_Cc76wp7YdDVI"),
		zap.WithChatID("-4672893880"),
	)
	if err != nil {
		panic(err)
	}
	return zapLogger
}
