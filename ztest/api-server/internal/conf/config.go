package conf

import (
	"encoding/json"
	"fmt"

	"github.com/jinzhu/copier"
	"github.com/r3labs/diff/v3"
	"github.com/yola1107/kratos/v2/config"
	"github.com/yola1107/kratos/v2/config/file"
	"github.com/yola1107/kratos/v2/library/log/zap"
	"github.com/yola1107/kratos/v2/log"
)

var (
	bc = &Bootstrap{}
)

func init() {}

func Init(flagconf string) config.Config {
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)

	if err := c.Load(); err != nil {
		panic(err)
	}

	// Unmarshal the config to struct
	if err := c.Scan(bc); err != nil {
		panic(err)
	}

	watch(c, "data")
	watch(c, "log")
	watch(c, "a")
	watchLogLevel(c)

	log.Infof("load config flagconf=%+v", flagconf)
	log.Infof("load config bc=%+v", ToJSON(bc))
	return c
}

func watch(c config.Config, key string) {
	err := c.Watch(key, func(key string, value config.Value) {
		newCfg := &Bootstrap{}
		if err := c.Scan(newCfg); err != nil {
			log.Errorf("Failed to scan new config: %v", err)
			return
		}
		changelog, _ := diff.Diff(bc, newCfg)
		for _, change := range changelog {
			fmt.Printf("Field=%s, from=%v to=%v\n", change.Path, change.From, change.To)
		}
		_ = copier.CopyWithOption(bc, newCfg, copier.Option{DeepCopy: true})
		log.Infof("watch：config(key=%s) changed: %+v\n", key, value.Load())
	})
	if err != nil {
		log.Errorf("Failed to watch config key=%s: %v", key, err)
	}
}

func watchLogLevel(c config.Config) {
	key := "log.level"
	if err := c.Watch(key, func(key string, value config.Value) {
		logger, ok := log.GetLogger().(*zap.Logger)
		if !ok {
			return
		}
		lv := value.Load().(string)
		if err := logger.SetLevel(lv); err != nil {
			log.Errorf("Failed to set log level(%+v): %v", lv, err)
		}
	}); err != nil {
		log.Errorf("Failed to watch config key=%s: %v", key, err)
	}
}

// ToJSON json string
func ToJSON(v interface{}) string {
	j, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(j)
}

func GetConfig() *Bootstrap {
	return bc
}
