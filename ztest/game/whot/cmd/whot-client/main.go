package main

import (
	"flag"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/game/whot/tools/press"
)

const (
	Name = "whot-client"
)

var (
	flagconf string
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, e.g. -conf config.yaml")
}

func main() {
	flag.Parse()

	c, bc := press.LoadConfig(flagconf)
	defer c.Close()

	logger := zap.NewLogger(bc.LoadTest.Log)
	defer logger.Close()

	runner := press.NewRunner(bc.LoadTest, logger)
	runner.Start()
	defer runner.Stop()

	app := kratos.New(
		kratos.Name(Name),
		kratos.Logger(logger),
	)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
