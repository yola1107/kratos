package main

import (
	"context"
	"fmt"
	"math/rand"
	"runtime/debug"
	"sync"
	"time"

	zap "github.com/yola1107/kratos/contrib/log/zap/v2"
	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware/recovery"
	v1 "github.com/yola1107/kratos/v2/transport/_sample/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/transport/grpc"
	"github.com/yola1107/kratos/v2/transport/http"
	"github.com/yola1107/kratos/v2/transport/websocket"
	"go.uber.org/zap/zapcore"

	etcd "github.com/yola1107/kratos/contrib/registry/etcd/v2"
	etcdv3 "go.etcd.io/etcd/client/v3"
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

func main() {

	etcdClient, err := etcdv3.New(etcdv3.Config{
		Endpoints: []string{"127.0.0.1:2379"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer etcdClient.Close()

	// 生产环境配置
	zapLogger := zap.New(&zap.Config{
		Mode:          zap.Development, //zap.Production,    // os.Getenv("APP_ENV")
		Level:         "debug",
		Directory:     "./logs",
		Filename:      "app.log",
		ErrorFilename: "app-error.log",
		MaxSize:       500,
		MaxAge:        30,
		Telegram: &zap.TelegramConfig{
			Enabled:     true,
			ChatID:      "-4587116707",
			Token:       "7587951172:AAGjhHeHE4kKmj3FtOxas0B9MlgQaKoqk9M",
			Threshold:   zapcore.ErrorLevel,
			QueueSize:   1000,
			RateLimit:   3 * time.Second,
			MaxBatchCnt: 20,
			MaxRetries:  2,
			Prefix:      "<" + Name + ">" + " ",
		},
	})
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
		kratos.Logger(zapLogger),               // 使用自定义 Logger
		kratos.Registrar(etcd.New(etcdClient)), // 注册中心 ETCD
	)

	//zapLogger.Close() //调试
	{
		for i := 0; i < rand.Int()%1000; i++ {
			//time.Sleep(time.Millisecond * time.Duration(rand.Int()%1000+500))
			log.Errorf("测试消息(%d)", i)
		}
		log.Errorf("测试消息(end)")

		//count := 0
		//for count < 10 {
		//	go func() {
		//		for i := 0; i < rand.Int()%10; i++ {
		//			//time.Sleep(time.Millisecond * time.Duration(rand.Int()%1000+500))
		//			log.Errorf("group(%d) 测试消息(%d) %+v", count, i, time.Now().Nanosecond()/1000)
		//			//log.Errorf("group(%d) 测试消息(%d) %+v", count, i, time.Now().Format("05.000"))
		//
		//			time.Sleep(time.Millisecond * time.Duration(rand.Int()%5))
		//		}
		//		count++
		//		time.Sleep(time.Second * time.Duration(rand.Int()%2))
		//	}()
		//	log.Errorf("测试消息(end)")
		//}

		defer func() {
			if r := recover(); r != nil {
				x := fmt.Sprintf("==>案发时发生分解拉萨附近爱上了放假哦文件 书法家欧萨附件是浪费十六分静安寺分厘卡撒酒疯 发生panic:%v , \n%s", r, debug.Stack())
				log.Errorf("%d %s", len(x), x)
				//log.Errorf("==>案发时发生分解拉萨附近爱上了放假哦文件 书法家欧萨附件是浪费十六分静安寺分厘卡撒酒疯 发生panic:%v , \n%s", r, debug.Stack())
			}
		}()
		//panic("abc")
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

//	// 生产环境配置
//	zapLogger := zap.New(&zap.Options{
//		Mode:          zap.Production, // os.Getenv("APP_ENV")
//		Level:         "debug",
//		Directory:     "./logs",
//		Filename:      "app.log",
//		ErrorFilename: "app-error.log",
//		MaxSize:       500,
//		MaxAge:        30,
//	})
//	defer zapLogger.Close()

//	log.With(log.GetLogger(),
//		"service", "your_service_name",
//		"version", "v1.0.0",
//	)

//func LoadOptions() (*Options, error) {
//	opts := DefaultOptions()
//
//	// 1. 环境变量覆盖
//	if err := env.Parse(opts); err != nil {
//		return nil, fmt.Errorf("parse env config failed: %w", err)
//	}
//
//	// 2. 配置文件加载（示例使用viper）
//	if err := viper.UnmarshalKey("logging", opts); err != nil {
//		return nil, fmt.Errorf("unmarshal config failed: %w", err)
//	}
//
//	// 3. 安全校验
//	if err := opts.Validate(); err != nil {
//		return nil, fmt.Errorf("config validation failed: %w", err)
//	}
//
//	// 4. 路径标准化
//	opts.Directory = filepath.Clean(opts.Directory)
//	if !filepath.IsAbs(opts.Directory) {
//		opts.Directory = filepath.Join(defaultBaseDir, opts.Directory)
//	}
//
//	return opts, nil
//}
//
//func InitLogger() *zap.Config {
//	m := os.Getenv("APP_LOGGER_MODE")
//
//	////生产服
//	if m == "pord" {
//		return &zap.Config{
//			Mode:          zap.Production,
//			Level:         "info", // 生产环境默认info级别
//			Directory:     "/var/log/app",
//			Filename:      "app.log",
//			ErrorFilename: "error.log",
//			MaxSize:       200, // 单个日志文件最大200MB
//			MaxAge:        7,   // 保留7天
//			MaxBackups:    10,  // 保留10个备份
//			FlushInterval: 3 * time.Second,
//			Compress:      true, // 启用压缩
//			LocalTime:     true,
//			QueueSize:     2048, // 增大队列缓冲
//			PoolSize:      512,  // 更大的对象池
//			SensitiveKeys: []string{"password", "token", "secret"},
//			Telegram: &zap.TelegramConfig{
//				Enabled:   true,
//				Token:     os.Getenv("TG_PROD_TOKEN"),
//				ChatID:    os.Getenv("TG_PROD_CHAT_ID"),
//				Threshold: zapcore.ErrorLevel,
//				QueueSize: 2048,            // 更大的报警队列
//				RateLimit: 5 * time.Second, // 消息报错间隔
//			},
//		}
//	}
//
//	return &zap.Config{
//		Mode:          zap.Development,
//		Level:         "debug", // 开发环境更详细日志
//		Directory:     "./logs",
//		Filename:      "app.log",
//		ErrorFilename: "error.log",
//		MaxSize:       50, // 较小文件大小
//		MaxAge:        7,  // 保留7天
//		MaxBackups:    3,  // 保留3个备份
//		FlushInterval: 1 * time.Second,
//		Compress:      false, // 开发环境不压缩
//		LocalTime:     true,
//		QueueSize:     512, // 较小队列
//		PoolSize:      128,
//		SensitiveKeys: []string{"password"},
//		Telegram: &zap.TelegramConfig{
//			Enabled:   true,
//			Token:     os.Getenv("TG_DEV_TOKEN"),
//			ChatID:    os.Getenv("TG_DEV_CHAT_ID"),
//			Threshold: zapcore.ErrorLevel,
//			QueueSize: 100,
//			RateLimit: 3000 * time.Millisecond,
//		},
//	}
//
//}
