package data

import (
	"context"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"github.com/yola1107/kratos/v2/log"

	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewDataRepo, NewRedis)

type dataRepo struct {
	data *Data
	log  *log.Helper
}

// NewDataRepo .
func NewDataRepo(data *Data, logger log.Logger) biz.DataRepo {
	return &dataRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// Data .
type Data struct {
	// TODO wrapped database client
	redis *redis.Client
}

// NewData .
func NewData(c *conf.Data, logger log.Logger, redis *redis.Client) (*Data, func(), error) {
	cleanup := func() {
		log.Info("closing the data resources")
		_ = redis.Close()
	}
	return &Data{redis: redis}, cleanup, nil
}

func NewRedis(c *conf.Data) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr: c.Redis.Addr,
		// Password: c.Password,
		// DB:       int(c.Db),
	})

	// 测试连接
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		panic("failed to connect to Redis: " + err.Error())
	}

	return rdb
}
