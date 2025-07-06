package press

import (
	"fmt"

	"github.com/yola1107/kratos/v2/config"
	"github.com/yola1107/kratos/v2/config/file"
	zconf "github.com/yola1107/kratos/v2/library/log/zap/conf"
	"github.com/yola1107/kratos/v2/log"
)

type (
	Bootstrap struct {
		Log   *zconf.Log
		Press Press
	}
	Press struct {
		Url      string
		Open     bool
		Num      int32
		Batch    int32
		Interval int32 // ms
		StartID  int64
		MinMoney float32
		MAxMoney float32
	}
)

// LoadConfig 加载配置
func LoadConfig(flagconf string) (config.Config, *Bootstrap) {
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)
	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(fmt.Errorf("bootstrap config invalid: %v", err))
	}
	if err := WatchConfig(c, &bc); err != nil {
		panic(err)
	}
	return c, &bc
}

func WatchConfig(c config.Config, bc *Bootstrap) error {
	if err := c.Watch("press", func(key string, value config.Value) {
		var newConfig Press
		if err := value.Scan(&newConfig); err != nil {
			log.Infof("watch error: %v\n", err)
			return
		}
		log.Infof("[Config Watch] %s changed to %v\n", key, newConfig)
		bc.Press = newConfig

	}); err != nil {
		return fmt.Errorf("watch failed: %w", err)
	}
	return nil
}
