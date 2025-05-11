package conf

import (
	"encoding/json"
	"fmt"

	"github.com/yola1107/kratos/v2/config"
	"github.com/yola1107/kratos/v2/config/file"
	"github.com/yola1107/kratos/v2/log"
)

var (
	bc = &Bootstrap{}
)

func Init(flagconf string) {
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

	log.Infof("load flagconf=%+v", flagconf)
	log.Infof("load bc=%+v", ToJSON(bc))
}

func watch(c config.Config, key string) {
	err := c.Watch(key, func(key string, value config.Value) {
		_ = c.Scan(&bc)
		fmt.Printf("watch：config(key=%s) changed: %+v\n", key, value.Load())
	})
	if err != nil {
		fmt.Printf("Watch err. %+v\n", err) //panic(err)
	}
}

func GetConfig() *Bootstrap {
	return bc
}

// ToJSON json string
func ToJSON(v interface{}) string {
	j, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(j)
}
