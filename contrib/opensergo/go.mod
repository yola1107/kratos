module github.com/yola1107/kratos/contrib/opensergo/v2

go 1.22

toolchain go1.24.2

require (
	github.com/opensergo/opensergo-go v0.0.0-20220331070310-e5b01fee4d1c
	github.com/yola1107/kratos/v2 v2.8.4
	golang.org/x/net v0.34.0
	google.golang.org/genproto/googleapis/api v0.0.0-20240528184218-531527333157
	google.golang.org/grpc v1.65.0
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/go-playground/form/v4 v4.2.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto v0.0.0-20231212172506-995d672761c0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240604185151-ef581f913117 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/yola1107/kratos/v2 => ../../
