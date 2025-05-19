package conf

import (
	"sync"
	"sync/atomic"

	"github.com/yola1107/kratos/v2/config"
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
)

var (
	ins   atomic.Pointer[Bootstrap] // 原子指针存储配置
	logMu sync.RWMutex              // 日志级别修改锁
)

// InitConfig 安全初始化配置（深拷贝）
func InitConfig(initial *Bootstrap) {
	if initial == nil {
		panic("nil initial config")
	}

	copyConfig := &Bootstrap{}
	if err := ext.DeepCopy(copyConfig, initial); err != nil {
		panic("failed to deep copy initial config: " + err.Error())
	}
	ins.Store(copyConfig)
}

// Watch 启动配置监听
func Watch(c config.Config) {
	watchKeys := []string{"log", "room", "log.level"}
	for _, key := range watchKeys {
		if err := c.Watch(key, configWatcher); err != nil {
			log.Errorf("watch key=%s err=%v", key, err)
			return
		}
	}
	return
}

// 应用增量配置变更
func configWatcher(key string, value config.Value) {
	oldConf := ins.Load()
	if oldConf == nil {
		log.Errorf("config key=%s err=%+v", key, "配置未初始化")
		return
	}

	// 防御性深拷贝
	newConf := &Bootstrap{}
	if err := ext.DeepCopy(newConf, oldConf); err != nil {
		log.Errorf("config key=%s err=%+v", key, err)
		return
	}

	// 按需更新配置字段
	switch key {
	case "log":
		if err := value.Scan(newConf.Log); err != nil {
			log.Errorf("config key=%s err=%+v", key, err)
			return
		}
	case "room":
		if err := value.Scan(newConf.Room); err != nil {
			log.Errorf("config key=%s err=%+v", key, err)
			return
		}
	case "log.level":
		lv, err := value.String()
		if err != nil {
			log.Errorf("config key=%s err=%+v", key, err)
			return
		}
		newConf.Log.Level = lv
		safeSetLogLevel(lv)
	default:
		return
	}

	if _, diff, _ := ext.DiffLog(oldConf, newConf); len(diff) > 0 {
		// 原子替换新配置
		ins.Store(newConf)
		log.Warnf("Config key=\"%s\" changed: \n%s", key, ext.ToJSONPretty(diff))
	}
	return
}

// 线程安全的日志级别修改
func safeSetLogLevel(lv string) {
	logMu.Lock()
	defer logMu.Unlock()

	logger, ok := log.GetLogger().(*zap.Logger)
	if !ok {
		log.Error("日志器类型不匹配")
		return
	}

	prevLevel := logger.GetLevel()
	if err := logger.SetLevel(lv); err != nil {
		log.Errorf("日志级别修改失败: %v (尝试从 %s 改为 %s)",
			err, prevLevel, lv)
		return
	}
	log.Infof("日志级别已变更: %s → %s", prevLevel, lv)
}

// GetConfig 配置获取方法
func GetConfig() *Bootstrap        { return ins.Load() }
func GetLogConfig() *Log           { return GetConfig().Log }
func GetRoomConfig() *Room         { return GetConfig().Room }
func GetTableConfig() *TableConfig { return GetConfig().Room.Table }
func GetGameConfig() *GameConfig   { return GetConfig().Room.Game }
func GetRobotConfig() *RobotConfig { return GetConfig().Room.Robot }
