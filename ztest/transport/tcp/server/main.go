package main

import (
	"context"
	"fmt"

	gproto "github.com/golang/protobuf/proto"
	"github.com/yola1107/kratos/v2/library/task"
	"github.com/yola1107/kratos/v2/metadata"
	v2 "github.com/yola1107/kratos/v2/ztest/transport/api/helloworld/v1"

	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware/recovery"
	"github.com/yola1107/kratos/v2/transport/grpc"
	"github.com/yola1107/kratos/v2/transport/http"
	"github.com/yola1107/kratos/v2/transport/tcp"
	//"google.golang.org/grpc/metadata"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	Name    = "helloworld"
	tcpLoop *task.Loop
)

type server struct {
	v2.UnimplementedGreeterServer
}

// var key string
var session *tcp.ChanList

func (s *server) SetCometChan(cl *tcp.ChanList, cs *tcp.Server) {
	session = cl
}

func (s *server) IsLoopFunc(f string) (isLoop bool) {
	m := map[string]bool{
		"SayHelloReq":  true,
		"SayHello2Req": true,
	}
	return m[f]
}

func (s *server) GetTCPLoop() task.ILoop {
	return tcpLoop
}

func (s *server) SayHelloReq(ctx context.Context, in *v2.HelloRequest) (*v2.HelloReply, error) {
	return &v2.HelloReply{Message: "SayHelloReq. Hello " + in.Name}, nil
}

func (s *server) SayHello2Req(ctx context.Context, in *v2.Hello2Request) (*v2.Hello2Reply, error) {
	//获取玩家的sessionID
	mid := ""
	// 从 ctx 中提取 metadata
	md, ok := metadata.FromServerContext(ctx)
	if !ok {
		fmt.Println("无法获取 metadata")
	} else {
		// 获取 mid
		mid = md.Get("mid")
	}
	resp := &v2.Hello2Reply{Message: "rsp_888888"}
	bytes, err := gproto.Marshal(resp)
	if err != nil {
		log.Errorf("err %+v", err.Error())
	}
	session.PushChan <- &tcp.PushData{
		Mid:  mid,
		Ops:  int32(v2.GameCommand_SayHello2Rsp),
		Data: bytes,
	}

	return &v2.Hello2Reply{Message: "SayHello2Req. Hello " + in.Name}, nil
}

func main() {
	tcpLoop = task.NewLoop(10000)
	tcpLoop.Start()
	defer tcpLoop.Stop()

	s := &server{}
	httpSrv := http.NewServer(
		http.Address(":8000"),
		http.Middleware(
			recovery.Recovery(),
		),
	)
	grpcSrv := grpc.NewServer(
		grpc.Address(":9000"),
		grpc.Middleware(
			recovery.Recovery(),
		),
	)
	tcpSrv := tcp.NewServer(
		tcp.Address(":3101"),
		tcp.Middleware(
			recovery.Recovery(),
		),
	)

	v2.RegisterGreeterServer(grpcSrv, s)
	v2.RegisterGreeterHTTPServer(httpSrv, s)
	v2.RegisterGreeterTCPServer(tcpSrv, s)
	app := kratos.New(
		kratos.Name(Name),
		kratos.Server(
			httpSrv,
			grpcSrv,
			tcpSrv,
		),
	)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
