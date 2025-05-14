package zap

import (
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/yola1107/kratos/v2/log"
)

func TestZapLogger(t *testing.T) {
	Name := "zap-test"
	zapLogger, err := NewLogger(
		//zap.WithProduction(),
		WithLevel("debug"),
		WithDirectory("./logs"),
		WithFilename(Name+".log"),
		WithErrorFilename(Name+"_error.log"),
		WithPrefix(Name),
		WithToken("token"),
		WithChatID("chat_id"),
		//WithToken("7945687310:AAHA9tkUPV1ELEsVSLoDZe_Cc76wp7YdDVI"),
		//WithChatID("-4672893880"),
		WithSensitiveKeys([]string{"pwd", "password"}))
	if err != nil {
		panic(err)
	}
	defer zapLogger.Close()

	log.SetLogger(zapLogger)

	log.Infof("")
	log.Info("SensitiveKeys. password=")

	log.Debugf("debug")
	log.Infof("info")
	log.Warnf("warn")
	log.Errorf("error")
	//log.Fatal("fatal")

	// 设置level
	log.Debugf("set level 1")
	_ = zapLogger.SetLevel("info")
	//log.GetLogger().(*Logger).SetLevel("info")
	log.Debugf("set level 2")

	// 方式A：直接使用 zap
	helperA := log.NewHelper(zapLogger.With(
		"user_id", 1001,
		"password", "sensitive_data", // 这个字段会被自动过滤
	))
	helperA.Infof("helper A")

	// 方式B：转换使用 zap
	helperB := log.NewHelper(log.GetLogger().(*Logger).With(
		"k", 1001,
		"password2", "sensitive_data2", // 这个字段会被自动过滤
	))
	helperB.Infof("helper B")

	// 测试消息
	for i := 0; i < 1; i++ {
		log.Errorf("测试消息(%d)", i)
	}
	log.Errorf("测试消息(end)")

	if true {
		go func() {
			incr := 0
			for {
				incr++
				log.Errorf("test %d", incr)
				time.Sleep(time.Duration(rand.Intn(1000)+50) * time.Millisecond)
			}
		}()
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c
		log.Infof("get a signal %s", s.String())
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			log.Info("exit")
			time.Sleep(time.Second)
			return
		case syscall.SIGHUP:
		default:
			return
		}
	}
}
