//go:build wireinject
// +build wireinject

package zap

import (
	"github.com/google/wire"
	"github.com/yola1107/kratos/v2/library/log/zap/conf"
)

// wireLogger init kratos logger application.
func wireLogger(c *conf.Log) *Logger {
	panic(wire.Build(providerSet, serviceSet))
}

// serviceSet provides build set for logger
var serviceSet = wire.NewSet(NewTelegram, NewAlert, newZapWrap, initLogger, wire.Bind(new(Sender), new(*Telegram)))

// providerSet provides configuration from *conf.Builder.
var providerSet = wire.NewSet(provideLoggerConf, provideTelegramConf, provideAlerterConf)

func provideLoggerConf(c *conf.Log) *conf.Logger     { return c.Logger }
func provideAlerterConf(c *conf.Log) *conf.Alerter   { return c.Alerter }
func provideTelegramConf(c *conf.Log) *conf.Telegram { return c.Telegram }
