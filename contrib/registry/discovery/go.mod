module github.com/yola1107/kratos/contrib/registry/discovery/v2

go 1.24.2

require (
	github.com/go-resty/resty/v2 v2.11.0
	github.com/pkg/errors v0.9.1
	github.com/yola1107/kratos/v2 v2.8.6
)

require golang.org/x/net v0.41.0 // indirect

replace github.com/yola1107/kratos/v2 => ../../../
