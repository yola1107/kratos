module github.com/yola1107/kratos/contrib/registry/zookeeper/v2

go 1.22

toolchain go1.24.2

require (
	github.com/go-zookeeper/zk v1.0.3
	github.com/yola1107/kratos/v2 v2.8.3
	golang.org/x/sync v0.10.0
)

replace github.com/yola1107/kratos/v2 => ../../../
