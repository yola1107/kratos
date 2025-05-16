package main

import (
	"flag"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	Name     = conf.Name
	Version  = conf.Version
	flagconf = "" // flagconf is the config flag.
	id, _    = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")
}

func main() {
	flag.Parse()

	c := conf.Init(flagconf)
	defer c.Close()

	logger := loadLogger(Name)
	defer logger.Close()

	testLog()

	//logger.Info("<UNK>")
	//log.Info("room started",
	//	"game_id", conf.GameID,
	//	"arena_id", conf.ArenaID,
	//	"server_id", conf.ServerID)

	app := kratos.New(
		kratos.Name(Name),
		kratos.Logger(logger), // 使用自定义 Logger
	)

	//room.Init
	r := room.Init()
	defer r.Close()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func loadLogger(Name string) *zap.Logger {
	c := conf.GetLC()
	if c == nil {
		panic("config is nil")
	}
	opts := []zap.Option{
		zap.WithProduction(),
		zap.WithDirectory(c.Directory),
		zap.WithFilename(Name + ".log"),
		zap.WithErrorFilename(Name + "_error.log"),
		zap.WithPrefix(Name),
		zap.WithMaxSizeMB(10), //10M
		zap.WithMaxAgeDays(1), //1天
		zap.WithMaxBackups(10),
		zap.WithCompress(true),
		zap.WithLocalTime(true),
		zap.WithToken(os.Getenv("TG_TOKEN")),
		zap.WithChatID(os.Getenv("TG_CHAT_ID")),
		//zap.WithToken("7945687310:AAHA9tkUPV1ELEsVSLoDZe_Cc76wp7YdDVI"),
		//zap.WithChatID("-4672893880"),
	}

	if os.Getenv("ENV_LOG_MODE") != "" {
		opts = append(opts, zap.WithProduction())
	}
	if c.Level != "" {
		opts = append(opts, zap.WithLevel(c.Level))
	}
	if c.Directory != "" {
		opts = append(opts, zap.WithDirectory(c.Directory))
	}
	if len(c.Sensitive) > 0 {
		opts = append(opts, zap.WithSensitiveKeys(c.Sensitive))
	}

	logger, err := zap.NewLogger(opts...)
	if err != nil {
		panic(err)
	}

	return logger
}

func testLog() {
	if true {
		return
	}

	go func() {
		incr := int64(0)
		for {
			if incr++; incr >= math.MaxInt64-1 {
				incr = 0
			}
			x := rand.Intn(10)
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

			time.Sleep(time.Duration(rand.Int()%20+100) * time.Millisecond)
		}
	}()

	go func() {
		incr := int64(0)
		for {
			if incr++; incr >= math.MaxInt64-1 {
				incr = 0
			}
			log.Errorf("error incr: (%d)", incr)
			time.Sleep(time.Duration(rand.Int()%20+1) * time.Millisecond)
		}
	}()
}
