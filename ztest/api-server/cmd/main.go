package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"reflect"
	"time"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/config"
	"github.com/yola1107/kratos/v2/config/file"
	"github.com/yola1107/kratos/v2/library/event"
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/library/log/zap"
	zconf "github.com/yola1107/kratos/v2/library/log/zap/conf"
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

	c, bc, lc, bus := loadConfig()
	defer c.Close()

	logger := zap.NewLogger(lc)
	log.SetLogger(logger)
	defer logger.Close()

	if err := watchConfig(c, bus, bc, lc, logger); err != nil {
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

func loadConfig() (config.Config, *conf.Bootstrap, *zconf.Bootstrap, *event.Bus) {
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)

	if err := c.Load(); err != nil {
		panic(err)
	}

	var (
		bc conf.Bootstrap
		lc zconf.Bootstrap
	)

	if err := c.Scan(&bc); err != nil || bc.ValidateAll() != nil {
		panic(fmt.Errorf("bootstrap config invalid: %v", err))
	}
	if err := c.Scan(&lc); err != nil || lc.ValidateAll() != nil {
		panic(fmt.Errorf("logger config invalid: %v", err))
	}

	return c, &bc, &lc, event.NewEventBus()
}

// 监听配置变更并推送事件
func watchConfig(c config.Config, bus *event.Bus, bc *conf.Bootstrap, lc *zconf.Bootstrap, logger *zap.Logger) error {
	// 订阅配置变更回调
	subscribeBusHandlers(bus, logger)

	for key, ptr := range map[string]any{
		"room.game":     bc.Room.Game,
		"room.robot":    bc.Room.Robot,
		"room.logCache": bc.Room.LogCache,
		"log.logger":    lc.Log.Logger,
		"log.alerter":   lc.Log.Alerter,
		"log.telegram":  lc.Log.Telegram,
	} {
		if err := c.Watch(key, observer(key, ptr, bus)); err != nil {
			return fmt.Errorf("watch %q failed: %w", key, err)
		}
	}
	return nil
}

func observer(key string, target any, bus *event.Bus) func(string, config.Value) {
	return func(_ string, val config.Value) {
		typ := reflect.TypeOf(target)
		if typ.Kind() != reflect.Pointer {
			log.Errorf("[config] %q target must be a pointer", key)
			return
		}

		newVal := reflect.New(typ.Elem()).Interface()
		if err := val.Scan(newVal); err != nil {
			log.Errorf("[config] scan failed: key=%q, err=%v", key, err)
			return
		}

		if v, ok := newVal.(interface{ ValidateAll() error }); ok {
			if err := v.ValidateAll(); err != nil {
				log.Errorf("[config] validation failed: key=%q, err=%v", key, err)
				return
			}
		}

		_, diff, err := ext.DiffLog(target, newVal)
		if err != nil {
			log.Errorf("[config] diff failed: key=%q, err=%v", key, err)
			return
		}
		if len(diff) > 0 {
			log.Warnf("[config] [%q] updated:\n%s", key, diff)
			// 刷新配置 深拷贝
			if err := ext.DeepCopy(target, newVal); err != nil {
				log.Errorf("[config] update failed: key=%q, err=%v", key, err)
				return
			}
			// 通知订阅者
			bus.Publish(key, newVal)
		}
	}
}

// 注册相关的订阅者回调
func subscribeBusHandlers(bus *event.Bus, logger *zap.Logger) {
	bus.Subscribe("log.logger", func(val any) {
		if v, ok := val.(*zconf.Logger); ok {
			if v.Level != logger.GetLevel() {
				logger.SetLevel(v.Level)
			}
			if changes, err := ext.Diff(v.Sensitive, logger.GetSensitive()); err == nil && len(changes) > 0 {
				logger.SetSensitive(v.Sensitive)
			}
		}
	})
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
