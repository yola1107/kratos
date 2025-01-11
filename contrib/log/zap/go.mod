module github.com/yola1107/kratos/contrib/log/zap/v2

go 1.22

toolchain go1.22.10

require (
	go.uber.org/zap v1.26.0
	github.com/yola1107/kratos/v2 v2.8.3
)

require go.uber.org/multierr v1.11.0 // indirect

replace github.com/yola1107/kratos/v2 => ../../../
