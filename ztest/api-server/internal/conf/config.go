package conf

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/yola1107/kratos/v2/config"
	"github.com/yola1107/kratos/v2/config/file"
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
)

const (
	ConfigKeyLog  = "log"
	ConfigKeyRoom = "room"
)

var (
	mu  sync.RWMutex
	ins atomic.Pointer[Bootstrap]
)

// InitConfig 加载配置文件并监听变更
func InitConfig(flagconf string) (config.Config, *Bootstrap) {
	c := config.New(
		config.WithSource(
			file.NewSource(fmt.Sprintf("%s", flagconf)),
		),
	)

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc = &Bootstrap{}
	if err := c.Scan(bc); err != nil {
		panic(err)
	}

	// 存储配置
	FillNilMessage(bc)
	ins.Store(bc)

	// 热更新配置
	Watch(c)

	log.Infof("config initialized: flagconf=%s", flagconf)
	return c, bc
}

// Watch 启动配置监听
func Watch(c config.Config) {
	for _, key := range []string{ConfigKeyLog, ConfigKeyRoom} {
		if err := c.Watch(key, func(key string, value config.Value) {
			handleConfigChange(key, value)
		}); err != nil {
			log.Errorf("config: watch %q failed: %v", key, err)
		}
	}
	return
}

func handleConfigChange(key string, value config.Value) {
	oldConf := ins.Load()
	newConf := &Bootstrap{}
	if err := ext.DeepCopy(newConf, oldConf); err != nil {
		log.Errorf("config: clone failed for key=%s: %v", key, err)
		return
	}

	// 处理更新
	switch key {
	case ConfigKeyLog:
		if err := value.Scan(newConf.Log); err != nil {
			log.Errorf("log config update failed: " + err.Error())
			return
		}
	case ConfigKeyRoom:
		if err := value.Scan(newConf.Room); err != nil {
			log.Errorf("room config update failed: " + err.Error())
			return
		}
	default:
		log.Warnf("config: unrecognized key %q", key)
		return
	}

	// 差异检测与原子存储
	FillNilMessage(newConf)
	if _, diff, _ := ext.DiffLog(oldConf, newConf); len(diff) > 0 {
		ins.Store(newConf)
		log.Warnf("Config changed:\n%s", ext.ToJSONPretty(diff))

		//log level 刷新
		if key == ConfigKeyLog && newConf.Log.Level != oldConf.Log.Level {
			safeSetLogLevel(newConf.Log.Level)
		}
	}
}

func safeSetLogLevel(lv string) {
	mu.Lock()
	defer mu.Unlock()

	if lv != "debug" && lv != "info" && lv != "warn" && lv != "error" {
		log.Errorf("invalid log level: %s", lv)
		return
	}

	logger, ok := log.GetLogger().(*zap.Logger)
	if !ok {
		log.Errorf("config: logger type assertion failed")
		return
	}

	current := logger.GetLevel()
	if err := logger.SetLevel(lv); err != nil {
		log.Errorf("set log level failed: %+v", err.Error())
		return
	}
	log.Infof("[config] log level changed from %q to %q", current, lv)
	return
}

// GetConfig 配置获取方法
func GetConfig() *Bootstrap  { return ins.Load() }
func GetLogConfig() *Log     { return GetConfig().Log }
func GetRoomConfig() *Room   { return GetConfig().Room }
func GetTableConfig() *Table { return GetConfig().Room.Table }
func GetGameConfig() *Game   { return GetConfig().Room.Game }
func GetRobotConfig() *Robot { return GetConfig().Room.Robot }
