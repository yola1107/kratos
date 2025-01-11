module github.com/yola1107/kratos/contrib/log/logrus/v2

go 1.22

toolchain go1.22.10

require (
	github.com/sirupsen/logrus v1.8.1
	github.com/yola1107/kratos/v2 v2.8.3
)

require golang.org/x/sys v0.21.0 // indirect

replace github.com/yola1107/kratos/v2 => ../../../
