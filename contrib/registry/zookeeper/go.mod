module github.com/yola1107/kratos/contrib/registry/zookeeper/v2

go 1.22

toolchain go1.22.10

require (
	github.com/yola1107/kratos/v2 v2.8.3
	github.com/go-zookeeper/zk v1.0.3
	golang.org/x/sync v0.8.0
)

replace github.com/yola1107/kratos/v2 => ../../../
