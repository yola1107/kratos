module github.com/yola1107/kratos/contrib/log/zerolog/v2

go 1.22

toolchain go1.22.10

require (
	github.com/rs/zerolog v1.30.0
	github.com/yola1107/kratos/v2 v2.8.3
)

require (
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	golang.org/x/sys v0.21.0 // indirect
)

replace github.com/yola1107/kratos/v2 => ../../../
