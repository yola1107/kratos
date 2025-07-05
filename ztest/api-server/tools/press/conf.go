package press

import (
	"fmt"

	"github.com/yola1107/kratos/v2/config"
	"github.com/yola1107/kratos/v2/config/file"
	zconf "github.com/yola1107/kratos/v2/library/log/zap/conf"
	"github.com/yola1107/kratos/v2/log"
)

type Bootstrap struct {
	Server struct {
		HTTP struct {
			Addr string `json:"addr"`
		} `json:"http"`
	} `json:"server"`
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

	if err := c.Scan(&bc); err != nil {
		panic(fmt.Errorf("bootstrap config invalid: %v", err))
	}
	if err := c.Scan(&lc); err != nil {
		panic(fmt.Errorf("logger config invalid: %v", err))
	}

	if err := WatchConfig(c, &bc); err != nil {
		panic(err)
	}

	return c, &bc, &lc
}

func WatchConfig(c config.Config, bc *Bootstrap) error {
	// 监听 http.addr 配置变化
	if err := c.Watch("server.http.addr", func(key string, value config.Value) {
		var newAddr string
		if err := value.Scan(&newAddr); err != nil {
			log.Infof("watch error: %v\n", err)
			return
		}
		log.Infof("[Config Watch] %s changed to %s\n", key, newAddr)
		bc.Server.HTTP.Addr = newAddr

	}); err != nil {
		return fmt.Errorf("watch http addr failed: %w", err)
	}

	return nil
}
