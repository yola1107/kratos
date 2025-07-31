package main

import (
	"flag"
	xhttp "net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/grpc"
	"github.com/yola1107/kratos/v2/transport/http"
	"github.com/yola1107/kratos/v2/transport/websocket"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/conf"
)

var (
	Name     = conf.Name
	Version  = conf.Version
	flagconf string // -conf path
	id, _    = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, e.g. -conf config.yaml")
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, ws *websocket.Server) *kratos.App {
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
	)
}

func main() {
	flag.Parse()

	go func() {
		runtime.GOMAXPROCS(runtime.NumCPU())
		runtime.SetBlockProfileRate(1) // 设置阻塞分析采样率 (每纳秒)
		log.Fatal(xhttp.ListenAndServe(":6060", nil))
	}()

	c, bc, lc := conf.LoadConfig(flagconf)
	defer c.Close()

	logger := zap.NewLogger(lc.Log)
	log.SetLogger(logger)
	defer logger.Close()

	if err := conf.WatchConfig(c, bc, lc, logger); err != nil {
		panic(err)
	}

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
