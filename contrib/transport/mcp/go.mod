module github.com/yola1107/kratos/contrib/transport/mcp/v2

go 1.23

toolchain go1.24.2

require (
	github.com/mark3labs/mcp-go v0.23.0
	github.com/yola1107/kratos/v2 v2.8.3
)

require (
	github.com/go-playground/form/v4 v4.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/yola1107/kratos/v2 => ../../../
