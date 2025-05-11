package conf

import (
	"flag"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/yola1107/kratos/v2/config"
	"github.com/yola1107/kratos/v2/config/file"
	"github.com/yola1107/kratos/v2/library/base"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
)

const GameID = 82003

const (
	logLevelKey = "log.level"
)

var (
	ArenaID  = 1  //场ID: 1 2 3 4
	ServerID = "" //房间ID
)

var (
	//Ins 配置实例  *Bootstrap
	ins atomic.Value

	public = &PublicConfig{}
)

func init() {
	flag.IntVar(&ArenaID, "aid", 1, "specify the arena ID. base.StrToInt(os.Getenv(\"ARENAID\"))")
	flag.StringVar(&ServerID, "sid", os.Getenv("HOSTNAME"), "specify the server ID.")
}

// Init 加载配置文件并监听变更
func Init(flagconf string) config.Config {

	c := config.New(
		config.WithSource(
			file.NewSource(fmt.Sprintf("%s/config.yaml", flagconf)),
			file.NewSource(fmt.Sprintf("%s/game_%d.yaml", flagconf, ArenaID)),
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

	_, diff, _ := base.DiffLog(oldCfg, newCfg)
	log.Warnf("Config changed. key=\"%s\" : %s", key, diff)
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
