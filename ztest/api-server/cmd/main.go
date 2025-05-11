package main

import (
	"flag"
	"os"

	"github.com/yola1107/kratos/v2"
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

	conf.Init(flagconf)

	//c := config.New(
	//	config.WithSource(
	//		file.NewSource(flagconf),
	//	),
	//)
	//defer c.Close()
	//
	//if err := c.Load(); err != nil {
	//	panic(err)
	//}
	//
	//var bc conf.Bootstrap
	//if err := c.Scan(&bc); err != nil {
	//	panic(err)
	//}

	app := kratos.New(
		kratos.Name(Name),
		//kratos.Logger(zapLogger), // 使用自定义 Logger
	)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
