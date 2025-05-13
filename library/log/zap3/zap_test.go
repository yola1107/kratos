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
	zapLogger, err := NewLogger()
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

	// 测试消息
	for i := 0; i < 1; i++ {
		log.Errorf("测试消息(%d)", i)
	}
	log.Errorf("测试消息(end)")

	go func() {
		incr := 0
		for {
			incr++
			//log.Debugf("test %d", incr)
			//log.Infof("test %d", incr)
			//log.Warnf("test %d", incr)
			//log.Errorf("test %d", incr)

			log.Errorf("test %d", incr)
			time.Sleep(time.Duration(rand.Intn(100)+50) * time.Millisecond)
		}
	}()

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
