package validate

import (
	"context"

	"github.com/yola1107/kratos/v2/errors"
	"github.com/yola1107/kratos/v2/middleware"
)

type validator interface {
	Validate() error
}

// Validator is a validator middleware.
//
// Deprecated: use github.com/yola1107/kratos/contrib/middleware/validate/v2.ProtoValidate instead.
func Validator() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (reply any, err error) {
			if v, ok := req.(validator); ok {
				if err := v.Validate(); err != nil {
					return nil, errors.BadRequest(errors.ValidatorReason, err.Error()).WithCause(err)
				}
			}
			return handler(ctx, req)
		}
	}
}
