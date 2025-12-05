module github.com/yola1107/kratos/contrib/log/zap/v2

go 1.24.2

require (
	github.com/yola1107/kratos/v2 v2.8.8
	go.uber.org/zap v1.27.0
)

require go.uber.org/multierr v1.11.0 // indirect

replace github.com/yola1107/kratos/v2 => ../../../
