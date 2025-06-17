package service

import (
	"github.com/google/wire"
	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(NewService)

// Service is a service.
type Service struct {
	v1.UnimplementedGreeterServer

	logger log.Logger
	uc     *biz.Usecase
}

// NewService new a service.
func NewService(uc *biz.Usecase, logger log.Logger, c *conf.Room) *Service {
	log.Infof("start server:\"%s\" version:%+v", conf.Name, conf.Version)
	log.Infof("GameID=%d ArenaID=%d ServerID=%s", conf.GameID, conf.ArenaID, conf.ServerID)

	return &Service{uc: uc, logger: logger}
}
