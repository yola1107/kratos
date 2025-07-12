package server

import (
	v1 "github.com/yola1107/kratos/v2/ztest/game/whot/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/service"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware/recovery"
	"github.com/yola1107/kratos/v2/transport/websocket"
)

// NewWebsocketServer new an Websocket server.
func NewWebsocketServer(c *conf.Server, greeter *service.Service, logger log.Logger) *websocket.Server {
	var opts = []websocket.ServerOption{
		websocket.Middleware(
			recovery.Recovery(),
		),
	}
	if c.Websocket.Network != "" {
		opts = append(opts, websocket.Network(c.Websocket.Network))
	}
	if c.Websocket.Addr != "" {
		opts = append(opts, websocket.Address(c.Websocket.Addr))
	}
	if c.Websocket.Timeout != nil {
		opts = append(opts, websocket.Timeout(c.Websocket.Timeout.AsDuration()))
	}
	srv := websocket.NewServer(opts...)
	v1.RegisterWhotWebsocketServer(srv, greeter)
	return srv
}
