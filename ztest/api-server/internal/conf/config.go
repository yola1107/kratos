package conf

import (
	"sync/atomic"

	"github.com/yola1107/kratos/v2/config"
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
)

const (
	logLevelKey = "log.level"
)

var (
	//Ins 配置实例  *Bootstrap
	ins atomic.Pointer[Bootstrap]
)

// 加载配置
func loadConfig(newConf *Bootstrap) {
	ins.Store(newConf)
}

// Watch 监听配置变更 热更新
func Watch(c config.Config, bc *Bootstrap) {
	loadConfig(bc)
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
	oldCfg := ins.Load()
	newCfg := Bootstrap{}
	if err := c.Scan(&newCfg); err != nil {
		log.Errorf("updated config err: %v", err)
		return
	}
	if _, diff, _ := ext.DiffLog(oldCfg, &newCfg); len(diff) > 0 {
		loadConfig(&newCfg)
		log.Warnf("Config key=\"%s\" changed: \n%s", key, ext.ToJSONPretty(diff))
	}
}

func refreshEvent(c config.Config, key string, value config.Value) {
	switch key {
	case logLevelKey:
		if lv, err := value.String(); err != nil {
			log.Errorf("log level set err:%v", value)
		} else {
			setLogLevel(lv)
		}
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

// 获取配置（只读访问）
func GetConfig() *Bootstrap {
	return ins.Load()
}

func GetLogConfig() *Log {
	return GetConfig().Log
}

func GetRoomConfig() *Room {
	return GetConfig().Room
}

func GetTableConfig() *TableConfig {
	return GetConfig().Room.Table
}

func GetGameConfig() *GameConfig {
	return GetConfig().Room.Game
}

func GetRobotConfig() *RobotConfig {
	return GetConfig().Room.Robot
}
