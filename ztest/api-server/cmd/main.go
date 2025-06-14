package main

import (
	"flag"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/grpc"
	"github.com/yola1107/kratos/v2/transport/http"
	"github.com/yola1107/kratos/v2/transport/websocket"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
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

	c, bc, lc := conf.LoadConfig(flagconf)
	defer c.Close()

	logger := zap.NewLogger(lc)
	log.SetLogger(logger)
	defer logger.Close()

	if err := conf.WatchConfig(c, bc, lc, logger); err != nil {
		panic(err)
	}

	// test...
	go testLog(logger)

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

// 测试日志（可用于压测日志模块）
func testLog(logger *zap.Logger) {
	if true {
		return
	}

	for i := 0; i < 5; i++ {
		go func(group int) {
			incr := int64(0)
			for {
				log.Debugf("GroupID=%d debug: ", i)
				log.Infof("GroupID=%d Infof: -", i)
				log.Warnf("GroupID=%d Warnf: -", i)
				// log.Errorf("GroupID=%d Errorf: -", i)

				if ext.IsHitFloat(0.000001) {
					log.Errorf("(%d) error %d", group, incr)
				}

				incr = (incr + 1) % math.MaxInt64
				time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
			}
		}(i)
	}
}
