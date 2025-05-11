package conf

import (
	"encoding/json"
	"fmt"

	"github.com/jinzhu/copier"
	"github.com/r3labs/diff/v3"
	"github.com/yola1107/kratos/v2/config"
	"github.com/yola1107/kratos/v2/config/file"
	"github.com/yola1107/kratos/v2/log"
)

var (
	bc = &Bootstrap{}
)

func init() {}

func Init(flagconf string) (config.Config, *Bootstrap) {
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

	log.Infof("load flagconf=%+v", flagconf)
	log.Infof("load bc=%+v", ToJSON(bc))

	return c, bc
}

func watch(c config.Config, key string) {
	err := c.Watch(key, func(key string, value config.Value) {
		newCfg := &Bootstrap{}
		if err := c.Scan(newCfg); err != nil {
			return
		}

		changelog, _ := diff.Diff(bc, newCfg)
		for _, change := range changelog {
			fmt.Printf("Field=%s, change=(%v ==> %v)\n", change.Path, change.From, change.To)
		}
		_ = copier.CopyWithOption(bc, newCfg, copier.Option{DeepCopy: true})
		fmt.Printf("watch：config(key=%s) changed: %+v\n\n", key, value.Load())
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
