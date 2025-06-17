package data

import (
	"github.com/google/wire"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewDataRepo)

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
	// db redis mq grpcClient...
}

// NewData .
func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	cleanup := func() {
		log.Info("closing the data resources")
	}
	return &Data{}, cleanup, nil
}
