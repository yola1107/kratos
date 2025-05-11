package logger

import (
	"os"

	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
)

func InitZapLogger(Name string) *zap.Logger {
	c := conf.Get().Log
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
