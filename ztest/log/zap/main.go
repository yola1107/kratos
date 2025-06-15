package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime/debug"
	"time"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/library/log/zap/conf"
	"github.com/yola1107/kratos/v2/log"
)

var (
	Name     = "log-server"
	Version  = "v0.0.1"
	flagconf string // -conf path
	id, _    = os.Hostname()
)

func main() {
	// logger := log.With(log.NewStdLogger(os.Stdout),
	// 	// "ts", log.DefaultTimestamp,
	// 	"ts", log.Timestamp("2006-01-02 15:04:05.000"),
	// 	"caller", log.DefaultCaller,
	//
	// 	// "service.id", id,
	// 	"service.name", Name,
	// 	// "service.version", Version,
	// 	"trace.id", tracing.TraceID(),
	// 	"span.id", tracing.SpanID(),
	// )
	// log.SetLogger(logger)

	logger := loadLog()
	defer logger.Close()

	app := kratos.New(
		kratos.Name(Name),
		kratos.Logger(logger),
	)

	testLog(logger)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func loadLog() *zap.Logger {
	return zap.NewLogger(conf.DefaultConfig(
		conf.WithProduction(),
		conf.WithAppName(Name),
		conf.WithLevel("debug"),
		conf.WithDirectory("./logs"),
		conf.WithSensitive([]string{"pwd", "password", "token"}),

		// 文件切割配置
		conf.WithMaxSizeMB(5),
		conf.WithMaxAgeDays(3),
		conf.WithMaxBackups(5),
		conf.WithCompress(true),
		conf.WithLocalTime(true),

		// Telegram 告警配置
		conf.WithAlertEnable(true),
		conf.WithAlertFormat("html"),
		conf.WithTelegramToken("7945687310:AAHA9tkUPV1ELEsVSLoDZe_Cc76wp7YdDVI"),
		conf.WithTelegramChatID("-4672893880"),
	))
}

// 演示日志功能：等级输出、脱敏、字段绑定等
func testLog(zapLogger *zap.Logger) {
	ctx := context.WithValue(context.Background(), "ctxKey", "ctxValue")
	log.Context(ctx).Infof("context value: %+v", ctx.Value("ctxKey"))

	// 基础日志级别
	log.Debugf("debug log")
	log.Infof("info log")
	log.Warnf("warn log")
	log.Errorf("error log")

	// 设置level
	zapLogger.SetLevel("info")
	zapLogger.GetZap().Debug("底层zapLogger Debug: abc") // 不会打印
	zapLogger.GetZap().Error("底层zapLogger Error: abc") // skipCaller = 3
	zapLogger.SetLevel("debug")

	// 脱敏示例
	h := log.NewHelper(log.NewFilter(zapLogger,
		// log.FilterLevel(log.LevelError),    // 仅输出 Error 及以上级别
		log.FilterKey("password", "token", "pwd"),
	))
	h.Infow("password", "123456", "token", "abc", "pwd", "abc123") // {"password": "***", "token": "***", "pwd": "***"}

	// 带字段日志示例 不绑定全局 help
	log.Infof("xxxxxxxxxxxxxxxxxx")
	h1 := log.NewHelper(log.With(zapLogger))
	h1.Infof("help")

	// 绑定全局字段示例
	log.SetLogger(log.With(zapLogger, "Version", Version, "Host", id))
	log.Infof("field log test")

	// 测试消息
	for i := 0; i < 1; i++ {
		log.Errorf("测试消息(%d)", i)
	}
	log.Errorf("测试消息(end)")

	if true {
		go func() {
			var i int64
			for {
				fmt.Printf("\n")
				log.Debugf("loop debug: %d", i)
				log.Infof("loop info: %d", i)
				log.Warnf("loop warn: %d", i)
				log.Errorf("loop error: %d", i)

				log.Errorf("error incr: (%d)", i)

				i = (i + 1) % math.MaxInt64
				time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
			}
		}()
	}
	defer func() {
		if r := recover(); r != nil {
			x := fmt.Sprintf("==>案发时发生分解拉萨附近爱上了放假哦文件 书法家欧萨附件是浪费十六分静安寺分厘卡撒酒疯 发生panic:%v , \n%s", r, debug.Stack())
			log.Errorf("%s", x)
		}
	}()
	panic("abc")
}
