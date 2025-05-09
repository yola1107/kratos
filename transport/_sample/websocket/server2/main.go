package main

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware/recovery"
	v1 "github.com/yola1107/kratos/v2/transport/_sample/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/transport/grpc"
	"github.com/yola1107/kratos/v2/transport/http"
	"github.com/yola1107/kratos/v2/transport/websocket"
	//"github.com/yola1107/kratos/contrib/log/zap/v2"
	//"github.com/yola1107/kratos/contrib/registry/etcd/v2"
	//etcdv3 "go.etcd.io/etcd/client/v3"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	Name = "helloworld"
)

type server struct {
	v1.UnimplementedGreeterServer

	sessionsMap sync.Map
}

func (s *server) IsLoopFunc(f string) bool {
	return false
}
func (s *server) SayHelloReq(ctx context.Context, in *v1.HelloRequest) (*v1.HelloReply, error) {
	return &v1.HelloReply{Message: in.Name}, nil
}

func (s *server) SayHello2Req(ctx context.Context, in *v1.Hello2Request) (*v1.Hello2Reply, error) {
	session := ctx.Value("session")
	if session != nil {
		v1.GetLoop().Post(func() {
			ss := session.(*websocket.Session)
			err := ss.Push(2, &v1.Hello2Reply{Message: "server push."})
			if err != nil {
				log.Infof("push err:%v", err)
			}
		})
	}
	return &v1.Hello2Reply{}, nil
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

	//logger := log.With(log.NewStdLogger(os.Stdout),
	//	//"ts", log.DefaultTimestamp,
	//	"ts", log.Timestamp("2006-01-02 15:04:05.000"),
	//	"caller", log.DefaultCaller,
	//
	//	//"service.id", id,
	//	"service.name", Name,
	//	//"service.version", Version,
	//	"trace.id", tracing.TraceID(),
	//	"span.id", tracing.SpanID(),
	//)
	//log.SetLogger(logger)

	//etcdClient, err := etcdv3.New(etcdv3.Config{
	//	Endpoints: []string{"127.0.0.1:2379"},
	//})
	//if err != nil {
	//	log.Fatal(err)
	//}
	//defer etcdClient.Close()

	zapLogger := zap.New(zap.DefaultConfig(
		zap.WithMode(zap.Development),
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
	))
	defer zapLogger.Close()

	//// zap logger
	//zapLogger := zap.New(nil)
	//defer zapLogger.Close()

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
	)

	//log.SetLogger(zapLogger)

	//zapLogger.Close() //调试
	{

		//log.SetLogger(zapLogger.With(
		//	"service.name", Name,
		//	"trace.id", "",
		//	"span.id", "",
		//))

		//log.SetLogger(log.GetLogger().(*zap.Logger).With("k1", "v1"))
		//log.Info("hello world 1")
		//log.Info("hello world 2")
		//
		//helper := log.GetLogger().(*zap.Logger).NewHelper("pwd", "auth")
		//helper.Info("help test 1")
		//log.Info("help test 2")
		//helper.Debugf("help test 3")
		//
		//// 设置level
		//log.Debugf("this is the debug log(1)")
		//log.GetLogger().(*zap.Logger).SetLevel("info")
		//log.Debugf("this is the debug log(2)")

		log.Errorf("")
		for i := 0; i < 10; i++ {
			log.Errorf("测试消息(%d)", i)
		}
		log.Errorf("测试消息(end)")

		defer func() {
			if r := recover(); r != nil {
				x := fmt.Sprintf("==>案发时发生分解拉萨附近爱上了放假哦文件 书法家欧萨附件是浪费十六分静安寺分厘卡撒酒疯 发生panic:%v , \n%s", r, debug.Stack())
				log.Errorf("%s", x)
				//log.Errorf("==>案发时发生分解拉萨附近爱上了放假哦文件 书法家欧萨附件是浪费十六分静安寺分厘卡撒酒疯 发生panic:%v , \n%s", r, debug.Stack())
			}
		}()
		//panic("abc")
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

//func initLogger() *zap.Logger {
//	//// 生产环境配置
//	//zapLogger := zap.New(&zap.Config{
//	//	Mode:          zap.Production, //zap.Production,    // os.Getenv("APP_ENV")
//	//	Level:         "debug",
//	//	Directory:     "./logs",
//	//	Filename:      "app.log",
//	//	ErrorFilename: "app-error.log",
//	//	MaxSize:       500,
//	//	MaxAge:        30,
//	//	Alert: zap.Alert{
//	//
//	//		Threshold:   zapcore.ErrorLevel,
//	//		QueueSize:   100,
//	//		MaxInterval: 5 * time.Second,
//	//		MaxBatchCnt: 10,
//	//		MaxRetries:  1,
//	//		Prefix:      fmt.Sprintf("<%s> ", Name),
//	//		Telegram: zap.Telegram{
//	//			Token:  "7945687310:AAHA9tkUPV1ELEsVSLoDZe_Cc76wp7YdDVI",
//	//			ChatID: "-4672893880",
//	//		},
//	//	},
//	//})
//	//return zapLogger
//
//	c := zap.DefaultConfig(
//		zap.WithMode(zap.Development), // os.Getenv("APP_ENV")
//		zap.WithDirectory("./logs"),
//		zap.WithFilename(Name+".log"),
//		zap.WithErrorFilename(Name+"_error.log"),
//		zap.WithPrefix(Name),
//		zap.WithToken(os.Getenv("TG_TOKEN")),
//		zap.WithChatID(os.Getenv("TG_CHAT_ID")),
//	)
//	zapLogger := zap.New(c)
//	return zapLogger
//}
