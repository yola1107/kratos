package main

import (
	"context"
	"flag"
	"os"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/grpc"
	"github.com/yola1107/kratos/v2/transport/http"
	"github.com/yola1107/kratos/v2/transport/websocket"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/room"
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

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, ws *websocket.Server, rr *room.Room) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
			hs,
			ws,
		),
		//通过 Kratos 的 App Hook（适合全生命周期统一管理）
		kratos.BeforeStart(func(ctx context.Context) error {
			log.Infof("start server \"%s\" version:%+v", Name, Version)
			log.Infof("GameID=%d ArenaID=%d ServerID=%s", conf.GameID, conf.ArenaID, conf.ServerID)
			go rr.Start()
			return nil
		}),
		kratos.AfterStop(func(ctx context.Context) error {
			rr.Close()
			return nil
		}),
	)
}

func main() {
	flag.Parse()

	c, bc := conf.InitConfig(flagconf)
	defer c.Close()

	logger := loadLogger(Name, bc.Log)
	defer logger.Close()

	app, cleanup, err := wireApp(bc.Server, bc.Data, bc.Room, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}

func loadLogger(Name string, lc *conf.Log) *zap.Logger {
	if lc == nil {
		panic("log config is nil")
	}
	opts := []zap.Option{
		zap.WithProduction(),
		zap.WithLevel("debug"),
		zap.WithDirectory(lc.Directory),
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
	if lc.Level != "" {
		opts = append(opts, zap.WithLevel(lc.Level))
	}
	if lc.Directory != "" {
		opts = append(opts, zap.WithDirectory(lc.Directory))
	}
	if len(lc.Sensitive) > 0 {
		opts = append(opts, zap.WithSensitiveKeys(lc.Sensitive))
	}

	logger, err := zap.NewLogger(opts...)
	if err != nil {
		panic(err)
	}
	return logger
}
