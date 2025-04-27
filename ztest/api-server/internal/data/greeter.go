package data

import (
	"context"

	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz"

	"github.com/yola1107/kratos/v2/log"
)

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

func (r *dataRepo) Save(ctx context.Context, g *biz.Greeter) (*biz.Greeter, error) {
	return g, nil
}

func (r *dataRepo) Update(ctx context.Context, g *biz.Greeter) (*biz.Greeter, error) {
	return g, nil
}

func (r *dataRepo) FindByID(context.Context, int64) (*biz.Greeter, error) {
	r.log.Infof("==> FindByID: %v", int64(1))
	return nil, nil
}
