package service

import (
	"github.com/google/wire"
	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/game/whot/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz"
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
func NewService(uc *biz.Usecase, logger log.Logger) *Service {
	return &Service{uc: uc, logger: logger}
}
