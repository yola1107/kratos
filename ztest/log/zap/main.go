package main

import (
	"fmt"
	"math"
	"math/rand"
	"runtime/debug"
	"time"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
)

const (
	Name = "hello-server"
)

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
		zap.WithProduction(),
		zap.WithLevel("debug"),
		zap.WithDirectory("./logs"),
		zap.WithFilename(Name+".log"),
		zap.WithErrorFilename(Name+"_error.log"),
		zap.WithMaxSizeMB(100), //10M
		zap.WithMaxAgeDays(1),  //1天
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
		log.Fatal(err)
	}
	return zapLogger
}

func testLog(zapLogger *zap.Logger) {

	//////调试
	//zapLogger.Info("zapLogger start")
	//zapLogger.Close()
	//zapLogger.Info("zapLogger end")

	log.Infof("")
	log.Debugf("debug")
	log.Infof("info")
	log.Warnf("warn")
	log.Errorf("error")
	//log.Fatal("fatal")
	log.Info("SensitiveKeys. password=")

	// 方式A：直接使用 zap
	helperA := log.NewHelper(zapLogger.With(
		"user_id", 1001,
		"password", "sensitive_data", // 这个字段会被自动过滤
	))
	helperA.Infof("helper A")

	// 方式B：直接使用 zap
	helperB := log.NewHelper(log.GetLogger().(*zap.Logger).With(
		"k", 1001,
		"password2", "sensitive_data2", // 这个字段会被自动过滤
	))
	helperB.Infof("helper B")

	logger := zapLogger.
		With("a", "a").
		With("b", "b")

	helper := log.NewHelper(logger)
	helper.Infof("first")

	logger = logger.With("c", "c") // 创建了一个新的 logger（包含 a, b, c）
	helper = log.NewHelper(logger)
	helper.Infof("second")

	// 设置level
	log.Debugf("set level 1")
	log.GetLogger().(*zap.Logger).SetLevel("info")
	log.Debugf("set level 2")

	// 测试消息
	for i := 0; i < 1; i++ {
		log.Errorf("测试消息(%d)", i)
	}
	log.Errorf("测试消息(end)")

	if true {
		go func() {
			incr := 0
			for {
				if incr++; incr >= math.MaxInt64-1 {
					incr = 0
				}
				x := rand.Intn(5)
				switch x {
				case 0:
					log.Debugf("debug incr:%d", incr)
				case 1:
					log.Infof("info incr:%d", incr)
				case 2:
					log.Warnf("warn incr:%d", incr)
				case 3:
					log.Errorf("error incr: (%d)", incr)
				}

				time.Sleep(time.Duration(rand.Int()%10+1) * time.Millisecond)

				//incr++
				//log.Errorf("test %d", incr)
				//time.Sleep(time.Duration(rand.Intn(100)+100) * time.Millisecond)
			}
		}()
	}
	defer func() {
		if r := recover(); r != nil {
			x := fmt.Sprintf("==>案发时发生分解拉萨附近爱上了放假哦文件 书法家欧萨附件是浪费十六分静安寺分厘卡撒酒疯 发生panic:%v , \n%s", r, debug.Stack())
			log.Errorf("%s", x)
		}
	}()
	//panic("abc")
}
