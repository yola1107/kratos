module github.com/yola1107/kratos/contrib/log/tencent/v2

go 1.24.2

require (
	github.com/tencentcloud/tencentcloud-cls-sdk-go v1.0.2
	github.com/yola1107/kratos/v2 v2.8.8
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/klauspost/compress v1.15.1 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	go.uber.org/atomic v1.9.0 // indirect
)

replace github.com/yola1107/kratos/v2 => ../../../
