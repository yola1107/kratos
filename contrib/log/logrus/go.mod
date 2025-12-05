module github.com/yola1107/kratos/contrib/log/logrus/v2

go 1.24.2

require (
	github.com/sirupsen/logrus v1.8.1
	github.com/yola1107/kratos/v2 v2.8.8
)

require golang.org/x/sys v0.33.0 // indirect

replace github.com/yola1107/kratos/v2 => ../../../
