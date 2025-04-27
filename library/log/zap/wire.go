//go:build wireinject
// +build wireinject

package zap

import (
	"github.com/google/wire"
	"github.com/yola1107/kratos/v2/library/log/zap/conf"
)

// wireLogger init kratos logger application.
func wireLogger(c *conf.Bootstrap) *Logger {
	panic(wire.Build(providerSet, serviceSet))
}

// serviceSet provides build set for logger
var serviceSet = wire.NewSet(NewTelegram, NewAlert, newZapWrap, initLogger, wire.Bind(new(Sender), new(*Telegram)))

// providerSet provides configuration from *conf.Builder.
var providerSet = wire.NewSet(provideLoggerConf, provideTelegramConf, provideAlerterConf)

func provideLoggerConf(c *conf.Bootstrap) *conf.Logger     { return c.Log.Logger }
func provideAlerterConf(c *conf.Bootstrap) *conf.Alerter   { return c.Log.Alerter }
func provideTelegramConf(c *conf.Bootstrap) *conf.Telegram { return c.Log.Telegram }
