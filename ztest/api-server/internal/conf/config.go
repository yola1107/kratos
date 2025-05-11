package conf

import (
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/r3labs/diff/v3"
	"github.com/yola1107/kratos/v2/config"
	"github.com/yola1107/kratos/v2/config/file"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
)

const (
	logLevelKey = "log.level"
)

var (
	ins atomic.Value // 单例配置 *Bootstrap
)

// Init 加载配置文件并监听热更新
func Init(flagconf string) config.Config {
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)

	if err := c.Load(); err != nil {
		panic(fmt.Sprintf("load config failed: %v", err))
	}

	var bs Bootstrap
	if err := c.Scan(&bs); err != nil {
		panic(fmt.Sprintf("scan config failed: %v", err))
	}
	setConfig(&bs)

	// 监听配置变更
	for _, key := range []string{"data", "log", "a"} {
		watch(c, key)
	}
	watchLogLevel(c)

	log.Infof("Config initialized: flagconf=%+v config=%s", flagconf, ToJSON(&bs))
	return c
}

func GetConfig() *Bootstrap {
	return ins.Load().(*Bootstrap)
}

func setConfig(bs *Bootstrap) {
	ins.Store(bs)
}

// watch 监听指定 key 的配置变更
func watch(c config.Config, key string) {
	if err := c.Watch(key, func(key string, value config.Value) {
		updateConfig(c, key, value)
	}); err != nil {
		log.Errorf("Failed to watch config key=%s: %v", key, err)
	}
}

// updateConfig 更新配置并打印变更内容
func updateConfig(c config.Config, key string, value config.Value) {
	newCfg := &Bootstrap{}
	if err := c.Scan(newCfg); err != nil {
		log.Errorf("Failed to scan updated config: %v", err)
		return
	}

	oldCfg := GetConfig()
	changelog, err := diff.Diff(oldCfg, newCfg)
	if err != nil {
		log.Errorf("Failed to diff config: %v", err)
		return
	}

	if len(changelog) == 0 {
		log.Infof("No changes detected for key=%s", key)
		return
	}

	fields := make([]string, 0, len(changelog))
	for _, change := range changelog {
		fields = append(fields, fmt.Sprintf("Field=%s, From=%v, To=%v", change.Path, change.From, change.To))
	}

	setConfig(newCfg)
	log.Warnf("Config key=%s changed: %s", key, ToJSON(fields))
}

func watchLogLevel(c config.Config) {
	if err := c.Watch(logLevelKey, func(key string, value config.Value) {
		logger, ok := log.GetLogger().(*zap.Logger)
		if !ok {
			return
		}
		lv := value.Load().(string)
		if err := logger.SetLevel(lv); err != nil {
			log.Errorf("Failed to set log level: %v", err)
		}
	}); err != nil {
		log.Errorf("Failed to watch config key=%s: %v", logLevelKey, err)
	}
}

// ToJSON 格式化为缩进 JSON 字符串
func ToJSON(v interface{}) string {
	j, err := json.Marshal(v)
	if err != nil {
		log.Errorf("Failed to marshal JSON: %v", err)
		return "{}"
	}
	return string(j)
}
