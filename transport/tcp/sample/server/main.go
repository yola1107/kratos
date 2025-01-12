package main

import (
	"context"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware/recovery"
	"github.com/yola1107/kratos/v2/transport/http"
	"github.com/yola1107/kratos/v2/transport/tcp"
	v1 "github.com/yola1107/kratos/v2/transport/tcp/sample/api/helloworld/v1"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name = "helloworld"
	// Version is the version of the compiled software.
	// Version = "v1.0.0"
)

// server is used to implement helloworld.GreeterServer.
type server struct {
	v1.UnimplementedGreeterServer
}

func (s *server) SayHelloReq(ctx context.Context, in *v1.HelloRequest) (*v1.HelloReply, error) {
	return &v1.HelloReply{Message: "SayHelloReq. Hello " + in.Name}, nil
}

func (s *server) SayHello2Req(ctx context.Context, in *v1.Hello2Request) (*v1.Hello2Reply, error) {
	return &v1.Hello2Reply{Message: "SayHello2Req. Hello " + in.Name}, nil
}

func (s *server) SetCometChan(cl *tcp.ChanList, cs *tcp.Server) {}

func (s *server) IsLoopFunc(f string) (isLoop bool) {
	return false
}

func main() {
	s := &server{}
	httpSrv := http.NewServer(
		http.Address(":8000"),
		http.Middleware(
			recovery.Recovery(),
		),
	)
	tcpSrv := tcp.NewServer(
		tcp.Address(":6000"),
		tcp.Middleware(
			recovery.Recovery(),
		),
	)
	v1.RegisterGreeterHTTPServer(httpSrv, s)
	v1.RegisterGreeterTCPServer(tcpSrv, s)
	app := kratos.New(
		kratos.Name(Name),
		kratos.Server(
			tcpSrv,
			httpSrv,
		),
	)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
