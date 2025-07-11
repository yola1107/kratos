package conf

import (
	"flag"
	"fmt"
	"os"
	"reflect"

	"github.com/yola1107/kratos/v2/config"
	"github.com/yola1107/kratos/v2/config/file"
	"github.com/yola1107/kratos/v2/library/event"
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/library/log/zap"
	zconf "github.com/yola1107/kratos/v2/library/log/zap/conf"
	"github.com/yola1107/kratos/v2/log"
)

const Name = "whot"
const Version = "v0.0.1"
const GameID = 131

var ArenaID = 1   // 场ID: 1 2 3 4
var ServerID = "" // 房间ID

func init() {
	flag.IntVar(&ArenaID, "aid", 1, "specify the arena ID. base.StrToInt(os.Getenv(\"ARENAID\"))")
	flag.StringVar(&ServerID, "sid", os.Getenv("HOSTNAME"), "specify the server ID.")
}

// LoadConfig 加载配置
func LoadConfig(flagconf string) (config.Config, *Bootstrap, *zconf.Bootstrap) {
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)

	if err := c.Load(); err != nil {
		panic(err)
	}

	var (
		bc Bootstrap
		lc zconf.Bootstrap
	)

	if err := c.Scan(&bc); err != nil || bc.ValidateAll() != nil {
		panic(fmt.Errorf("bootstrap config invalid: %v", err))
	}
	if err := c.Scan(&lc); err != nil || lc.ValidateAll() != nil {
		panic(fmt.Errorf("logger config invalid: %v", err))
	}

	return c, &bc, &lc
}

// WatchConfig 监听配置变更并推送事件
func WatchConfig(c config.Config, bc *Bootstrap, lc *zconf.Bootstrap, logger *zap.Logger) error {
	// 定义事件总线
	bus := event.NewEventBus()

	// 订阅配置变更事件回调
	subscribeBus(bus, logger)

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
func subscribeBus(bus *event.Bus, logger *zap.Logger) {
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
