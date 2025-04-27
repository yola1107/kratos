module github.com/yola1107/kratos/contrib/encoding/msgpack/v2

go 1.22

toolchain go1.24.2

require (
	github.com/vmihailenco/msgpack/v5 v5.4.1
	github.com/yola1107/kratos/v2 v2.8.3
)

require github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect

replace github.com/yola1107/kratos/v2 => ../../../
