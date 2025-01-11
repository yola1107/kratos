module github.com/yola1107/kratos/contrib/registry/discovery/v2

go 1.22

toolchain go1.22.10

require (
	github.com/yola1107/kratos/v2 v2.8.3
	github.com/go-resty/resty/v2 v2.11.0
	github.com/pkg/errors v0.9.1
)

require golang.org/x/net v0.26.0 // indirect

replace github.com/yola1107/kratos/v2 => ../../../
