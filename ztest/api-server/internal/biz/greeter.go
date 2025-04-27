package biz

import (
	"context"

	"github.com/yola1107/kratos/v2/errors"
	"github.com/yola1107/kratos/v2/log"
)

var (
	// ErrUserNotFound is user not found.
	ErrUserNotFound = errors.NotFound("USER_NOT_FOUND", "user not found")
)

// Greeter is a Greeter model.
type Greeter struct {
	Hello string
}

// DataRepo is a data repo.
type DataRepo interface {
	Save(context.Context, *Greeter) (*Greeter, error)
	Update(context.Context, *Greeter) (*Greeter, error)
	FindByID(context.Context, int64) (*Greeter, error)
}

// DataUsecase is a Data usecase.
type DataUsecase struct {
	repo DataRepo
	log  *log.Helper
}

// NewDataUsecase new a data usecase.
func NewDataUsecase(repo DataRepo, logger log.Logger) *DataUsecase {
	return &DataUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *DataUsecase) GetDataRepo() DataRepo {
	return uc.repo
}

// CreateGreeter creates a Greeter, and returns the new Greeter.
func (uc *DataUsecase) CreateGreeter(ctx context.Context, g *Greeter) (*Greeter, error) {
	uc.log.Infof("CreateGreeter: %v", g.Hello)
	return uc.repo.Save(ctx, g)
}
