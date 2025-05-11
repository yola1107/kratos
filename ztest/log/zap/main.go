package main

import (
	"fmt"
	"math/rand"
	"runtime/debug"
	"time"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/library/log/zap"
	//"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
)

const (
	Name = "hello-server"
)

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

	zapLogger := loadLog()
	defer zapLogger.Close()

	app := kratos.New(
		kratos.Name(Name),
		kratos.Logger(zapLogger), // 使用自定义 Logger
	)

	testLog(zapLogger)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func loadLog() *zap.Logger {
	zapLogger, err := zap.NewLogger(
		//zap.WithDevelopment(),
		zap.WithProduction(),
		zap.WithLevel("debug"),
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
		zap.WithSensitiveKeys([]string{"pwd", "password"}),
	)
	if err != nil {
		log.Fatal(err)
	}
	return zapLogger
}

func testLog(zapLogger *zap.Logger) {

	//////调试
	//////zapLogger.Info("zapLogger start")
	//////zapLogger.Close()
	//////zapLogger.Info("zapLogger end")
	//
	//// 使用with
	//log.SetLogger(zapLogger.With(
	//	"service.name", Name,
	//	"trace.id", "",
	//	"span.id", "",
	//))

	//// 使用with
	//log.SetLogger(log.GetLogger().(*zap.Logger).With("password", "abc"))
	//log.Info("with 1")
	//log.Info("with 2")
	////
	//// 使用help log
	//helper := log.GetLogger().(*zap.Logger).NewHelper("pwd", "auth")
	//helper.Info("help 1")
	//log.Info("help 2")
	//helper.Debugf("help 3")

	//// 设置level
	//log.Debugf("set level 1")
	//log.GetLogger().(*zap.Logger).SetLevel("info")
	//log.Debugf("set level 2")

	log.Info("SensitiveKeys. password=")

	log.Debugf("debug")
	log.Infof("info")
	log.Warnf("warn")
	log.Errorf("error")
	//log.Fatal("fatal")

	// 测试消息
	for i := 0; i < 1; i++ {
		log.Errorf("测试消息(%d)", i)
	}
	log.Errorf("测试消息(end)")

	go func() {
		incr := 0
		for {
			incr++
			log.Infof("test %d", incr)
			time.Sleep(time.Duration(rand.Intn(500)+50) * time.Millisecond)
		}
	}()

	defer func() {
		if r := recover(); r != nil {
			x := fmt.Sprintf("==>案发时发生分解拉萨附近爱上了放假哦文件 书法家欧萨附件是浪费十六分静安寺分厘卡撒酒疯 发生panic:%v , \n%s", r, debug.Stack())
			log.Errorf("%s", x)
		}
	}()

	//panic("abc")
}
