package main

import (
	"context"
	"fmt"
	"log"
	"time"

	//"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware/recovery"
	transgrpc "github.com/yola1107/kratos/v2/transport/grpc"
	transhttp "github.com/yola1107/kratos/v2/transport/http"
	"github.com/yola1107/kratos/v2/transport/tcp"
	v1 "github.com/yola1107/kratos/v2/transport/tcp/sample/api/helloworld/v1"
	"google.golang.org/grpc"
)

var (
	seed = int64(0)
)

func main() {
	//http
	connHTTP, err := transhttp.NewClient(
		context.Background(),
		transhttp.WithMiddleware(
			recovery.Recovery(),
		),
		transhttp.WithEndpoint("127.0.0.1:8000"),
	)
	if err != nil {
		panic(err)
	}
	defer connHTTP.Close()

	// grpc
	connGRPC, err := transgrpc.DialInsecure(
		context.Background(),
		transgrpc.WithEndpoint("127.0.0.1:9000"),
		transgrpc.WithMiddleware(
			recovery.Recovery(),
		),
	)
	if err != nil {
		panic(err)
	}
	defer connGRPC.Close()

	//tcp
	u := &user{}
	if err = u.init(); err != nil {
		panic(err)
	}
	defer u.game.Close()

	for {

		log.Printf("-----------\n")
		callHTTP(connHTTP)
		callGRPC(connGRPC)
		callTCP(u)
		seed++

		time.Sleep(time.Second * 10)
	}
}

func callHTTP(connHTTP *transhttp.Client) {
	client := v1.NewGreeterHTTPClient(connHTTP)
	reply, err := client.SayHelloReq(context.Background(), &v1.HelloRequest{Name: "kratos_http"})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("[http] SayHello %s", reply.Message)
}

func callGRPC(connGRPC *grpc.ClientConn) {
	client := v1.NewGreeterClient(connGRPC)
	reply, err := client.SayHelloReq(context.Background(), &v1.HelloRequest{Name: "kratos_grpc"})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("[grpc] SayHello %s", reply.Message)
}

func callTCP(u *user) {
	err := u.game.Request(int32(v1.GameCommand_SayHello2Req), &v1.Hello2Request{Name: fmt.Sprintf("kratos_tcp:%d", seed)})
	if err != nil {
		log.Fatal(err)
	}
	//log.Printf("[TCP] SayHello2Req %v", u.ID)
}

type user struct {
	ID int64

	game *tcp.Client

	pushes map[int32]tcp.PushMsgHandle
	respes map[int32]tcp.RespMsgHandle
}

func (u *user) init() (err error) {

	//推送消息 push
	u.pushes = map[int32]tcp.PushMsgHandle{
		int32(v1.GameCommand_SayHelloRsp):  func(data []byte) { log.Printf("[tcp] SayHello. %v", string(data)) },
		int32(v1.GameCommand_SayHello2Rsp): func(data []byte) { log.Printf("[tcp] SayHello2. %v", string(data)) },
	}

	// 请求回复
	u.respes = map[int32]tcp.RespMsgHandle{
		int32(v1.GameCommand_SayHelloReq):  func(data []byte, code int32) { log.Printf("[TCP] SayHello. data=%+v code=%d", string(data), code) },
		int32(v1.GameCommand_SayHello2Req): func(data []byte, code int32) { log.Printf("[TCP] SayHello2. data=%+v code=%d", string(data), code) },
	}

	u.game, err = tcp.NewTcpClient(&tcp.ClientConfig{
		Addr:           ":6000",
		PushHandlers:   u.pushes,
		RespHandlers:   u.respes,
		DisconnectFunc: func() { log.Printf("DisconnectFunc %d", u.ID) },
		Token:          "",
	})

	return err
}
