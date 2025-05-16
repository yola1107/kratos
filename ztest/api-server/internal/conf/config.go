package conf

import (
	"fmt"
	"sync/atomic"

	"github.com/yola1107/kratos/v2/config"
	"github.com/yola1107/kratos/v2/config/file"
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
)

const (
	logLevelKey = "log.level"
)

var (
	//Ins 配置实例  *Bootstrap
	ins atomic.Value
)

// Init 加载配置文件并监听变更
func Init(flagconf string) config.Config {
	c := config.New(
		config.WithSource(
			file.NewSource(fmt.Sprintf("%s", flagconf)),
		),
	)

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}

	// 加载配置
	set(&bc)

	// 热更新配置
	watch(c)

	log.Infof("config initialized: flagconf=%s config=%+v", flagconf, ext.ToJSON(get()))
	return c
}

// 设置当前配置
func set(bs *Bootstrap) {
	ins.Store(bs)
}

// Get 获取当前配置
func get() *Bootstrap {
	v, ok := ins.Load().(*Bootstrap)
	if !ok {
		return &Bootstrap{}
	}
	return v
}

func watch(c config.Config) {
	for _, key := range []string{"log", "room", logLevelKey} {
		if err := c.Watch(key, func(key string, value config.Value) {
			updateConfig(c, key, value)
			refreshEvent(c, key, value)
		}); err != nil {
			log.Errorf("watch config key=%s failed: %v", key, err)
		}
	}
}

// updateConfig 扫描并比较变更，保存新配置
func updateConfig(c config.Config, key string, v config.Value) {
	oldCfg := get()
	newCfg := &Bootstrap{}
	if err := c.Scan(newCfg); err != nil {
		log.Errorf("updated config err: %v", err)
		return
	}
	if _, diff, _ := ext.DiffLog(oldCfg, newCfg); len(diff) > 0 {
		set(newCfg)
		log.Warnf("Config key=\"%s\" changed: \n%s", key, ext.ToJSONPretty(diff))
	}
}

func refreshEvent(c config.Config, key string, value config.Value) {
	switch key {
	case logLevelKey:
		setLogLevel(value.Load().(string))
	}
}

func setLogLevel(lv string) {
	logger, ok := log.GetLogger().(*zap.Logger)
	if !ok {
		return
	}
	if err := logger.SetLevel(lv); err != nil {
		log.Errorf("Failed to set log level: %v", err)
		return
	}
	log.Infof("success set logger level to \"%s\"", lv)
}

func GetBS() *Bootstrap {
	return get()
}

func GetLC() *Log {
	return get().Log
}

func GetTC() *TableConfig {
	return get().Room.Table
}

func GetGC() *GameConfig {
	return get().Room.Game
}

func GetRbC() *RobotConfig {
	return get().Room.Robot
}
