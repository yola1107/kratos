package conf

import (
	"fmt"
	"sync/atomic"

	"github.com/yola1107/kratos/v2/config"
	"github.com/yola1107/kratos/v2/config/file"
	"github.com/yola1107/kratos/v2/library/base"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
)

const logLevelKey = "log.level"

var ins atomic.Value // 保存 *Bootstrap 配置实例

// Init 加载配置文件并监听变更
func Init(flagconf string) config.Config {
	c := config.New(config.WithSource(
		file.NewSource(flagconf),
	))

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

	log.Infof("config initialized: flagconf=%s config=%+v", flagconf, base.ToJSON(Get()))
	return c
}

// 设置当前配置
func set(bs *Bootstrap) {
	ins.Store(bs)
}

// Get 获取当前配置
func Get() *Bootstrap {
	if v, ok := ins.Load().(*Bootstrap); !ok {
		return &Bootstrap{}
	} else {
		return v
	}
}

func watch(c config.Config) {
	for _, key := range []string{"data", "log", "a", logLevelKey} {
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
	oldCfg := Get()
	newCfg := &Bootstrap{}
	if err := c.Scan(newCfg); err != nil {
		log.Errorf("Failed to scan updated config: %v", err)
		return
	}
	set(newCfg)

	changelog, _ := base.Diff(oldCfg, newCfg)
	if len(changelog) == 0 {
		return
	}
	fields := make([]string, 0, len(changelog))
	for _, change := range changelog {
		fields = append(fields, fmt.Sprintf("Field=%s, From=%v, To=%v", change.Path, change.From, change.To))
	}
	log.Warnf("Config changed. key=\"%s\" : %s", key, fields)
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
