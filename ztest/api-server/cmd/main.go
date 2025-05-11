package main

import (
	"flag"
	"os"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	Name     string = "api-server"
	Version  string = "v0.0.0"
	flagconf string = "" // flagconf is the config flag.
	id, _           = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")
}

func main() {
	flag.Parse()

	c := conf.Init(flagconf)
	defer c.Close()

	zapLogger := loadLogger()
	defer zapLogger.Close()

	app := kratos.New(
		kratos.Name(Name),
		kratos.Logger(zapLogger), // 使用自定义 Logger
	)

	log.Debugf("debug")
	log.Infof("info")
	log.Warnf("warn")
	log.Errorf("error")

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func loadLogger() *zap.Logger {
	c := conf.GetConfig().Log
	if c == nil {
		panic("config is nil")
	}
	opts := []zap.Option{
		zap.WithDevelopment(),
		zap.WithDirectory(c.Directory),
		zap.WithFilename(Name + ".log"),
		zap.WithErrorFilename(Name + "_error.log"),
		zap.WithPrefix(Name),
		//zap.WithToken(os.Getenv("TG_TOKEN")),
		//zap.WithChatID(os.Getenv("TG_CHAT_ID")),
		zap.WithToken("7945687310:AAHA9tkUPV1ELEsVSLoDZe_Cc76wp7YdDVI"),
		zap.WithChatID("-4672893880"),
	}
	if os.Getenv("ENV_LOG_MODE") == string(zap.Production) {
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
	zapLogger, err := zap.NewLogger(opts...)
	if err != nil {
		panic(err)
	}
	return zapLogger
}
